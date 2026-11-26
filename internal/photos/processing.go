package photos

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	_ "golang.org/x/image/webp"
)

type ProcessedImage struct {
	Preview []byte
	Thumb   []byte
	Width   int32
	Height  int32
}

func decodeAndProcess(ctx context.Context, r io.Reader, maxMegapixels int, businessName string) (ProcessedImage, error) {
	// Read all bytes so we can DecodeConfig and decode; originals are capped at MAX_PHOTO_BYTES.
	data, err := io.ReadAll(r)
	if err != nil {
		return ProcessedImage{}, fmt.Errorf("read image: %w", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ProcessedImage{}, errInvalidImage("undecodable image")
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return ProcessedImage{}, errInvalidImage("invalid dimensions")
	}
	maxMP := maxMegapixels * 1_000_000
	if cfg.Width*cfg.Height > maxMP {
		return ProcessedImage{}, errInvalidImage("image too large")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ProcessedImage{}, errInvalidImage("undecodable image")
	}
	orientation := readJPEGOrientation(data)
	// Keep original bytes only as long as needed for EXIF; allow GC before large transforms.
	data = nil

	img = applyOrientation(img, orientation)
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w*h > maxMP {
		return ProcessedImage{}, errInvalidImage("image too large")
	}

	previewImg := resizeToLongestSide(img, 1600)
	img = nil
	watermarked := watermarkPreview(previewImg, businessName)
	thumbImg := resizeToLongestSide(watermarked, 480)

	var previewBuf, thumbBuf bytes.Buffer
	if err := jpeg.Encode(&previewBuf, watermarked, &jpeg.Options{Quality: 72}); err != nil {
		return ProcessedImage{}, fmt.Errorf("encode preview: %w", err)
	}
	if err := jpeg.Encode(&thumbBuf, thumbImg, &jpeg.Options{Quality: 72}); err != nil {
		return ProcessedImage{}, fmt.Errorf("encode thumb: %w", err)
	}
	return ProcessedImage{Preview: previewBuf.Bytes(), Thumb: thumbBuf.Bytes(), Width: int32(w), Height: int32(h)}, nil
}

type invalidImageError struct{ msg string }

func (e invalidImageError) Error() string { return e.msg }
func errInvalidImage(msg string) error    { return invalidImageError{msg: msg} }
func isInvalidImage(err error) bool {
	_, ok := err.(invalidImageError)
	return ok
}

func resizeToLongestSide(src image.Image, longest int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	if w <= longest && h <= longest {
		// Copy to RGBA to normalize.
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
		return dst
	}
	var nw, nh int
	if w >= h {
		nw = longest
		nh = int(math.Round(float64(h) * float64(longest) / float64(w)))
	} else {
		nh = longest
		nw = int(math.Round(float64(w) * float64(longest) / float64(h)))
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func watermarkPreview(src image.Image, businessName string) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)

	label := "PREVIEW"
	if strings.TrimSpace(businessName) != "" {
		label = "PREVIEW  " + strings.TrimSpace(businessName)
	}
	// Simple tiled diagonal watermark. Basicfont is small; draw scaled by repeating.
	tile := 260
	for y := -h; y < h*2; y += tile {
		for x := -w; x < w*2; x += tile {
			drawLabel(dst, x+y/2, y, label)
		}
	}
	// subtle dark veil for legibility
	veil := image.NewUniform(color.RGBA{0, 0, 0, 18})
	draw.Draw(dst, dst.Bounds(), veil, image.Point{}, draw.Over)
	// redraw source lightly? no: keep simple and visible
	return dst
}

func drawLabel(img *image.RGBA, x, y int, label string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.RGBA{255, 255, 255, 90}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(label)
}

// applyOrientation handles common EXIF orientation. We strip metadata on output by
// re-encoding JPEG without EXIF.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// convert to RGBA
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)

	var dst *image.RGBA
	switch orientation {
	case 2: // flip horizontal
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(w-1-x, y, src.RGBAAt(x, y))
			}
		}
	case 3: // rotate 180
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(w-1-x, h-1-y, src.RGBAAt(x, y))
			}
		}
	case 4: // flip vertical
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(x, h-1-y, src.RGBAAt(x, y))
			}
		}
	case 5: // transpose
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(y, x, src.RGBAAt(x, y))
			}
		}
	case 6: // rotate 90 CW
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(h-1-y, x, src.RGBAAt(x, y))
			}
		}
	case 7: // transverse
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(y, w-1-x, src.RGBAAt(x, y))
			}
		}
	case 8: // rotate 270 CW
		dst = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				dst.SetRGBA(y, w-1-x, src.RGBAAt(x, y))
			}
		}
	default:
		return img
	}
	return dst
}

// readJPEGOrientation parses EXIF orientation from JPEG APP1 marker. Minimal parser,
// returns 0 if not found.
func readJPEGOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 0
	}
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		i += 2
		if marker == 0xD9 || marker == 0xDA { // EOI/SOS
			break
		}
		if i+2 > len(data) {
			break
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		if segLen < 2 || i+segLen > len(data) {
			break
		}
		seg := data[i+2 : i+segLen]
		if marker == 0xE1 && len(seg) >= 6 && string(seg[:6]) == "Exif\x00\x00" {
			if o := parseTIFFOrientation(seg[6:]); o != 0 {
				return o
			}
		}
		i += segLen
	}
	return 0
}

func parseTIFFOrientation(tiff []byte) int {
	if len(tiff) < 8 {
		return 0
	}
	var bo binaryOrder
	switch string(tiff[:2]) {
	case "II":
		bo = littleEndian{}
	case "MM":
		bo = bigEndian{}
	default:
		return 0
	}
	if bo.Uint16(tiff[2:4]) != 42 {
		return 0
	}
	ifd := int(bo.Uint32(tiff[4:8]))
	if ifd < 0 || ifd+2 > len(tiff) {
		return 0
	}
	n := int(bo.Uint16(tiff[ifd : ifd+2]))
	entry := ifd + 2
	for j := 0; j < n && entry+12 <= len(tiff); j++ {
		tag := bo.Uint16(tiff[entry : entry+2])
		typ := bo.Uint16(tiff[entry+2 : entry+4])
		count := bo.Uint32(tiff[entry+4 : entry+8])
		if tag == 0x0112 && count == 1 { // Orientation
			var val uint16
			if typ == 3 { // SHORT
				val = bo.Uint16(tiff[entry+8 : entry+10])
			}
			if val >= 1 && val <= 8 {
				return int(val)
			}
		}
		entry += 12
	}
	return 0
}

type binaryOrder interface {
	Uint16([]byte) uint16
	Uint32([]byte) uint32
}
type littleEndian struct{}
type bigEndian struct{}

func (littleEndian) Uint16(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 }
func (littleEndian) Uint32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
func (bigEndian) Uint16(b []byte) uint16 { return uint16(b[1]) | uint16(b[0])<<8 }
func (bigEndian) Uint32(b []byte) uint32 {
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
}

// Suppress unused import warning in some builds.
var _ = png.Decode
