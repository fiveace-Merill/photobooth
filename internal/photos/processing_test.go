package photos

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func makeJPEG(t *testing.T, w, h int, withEXIF bool) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if !withEXIF {
		return b
	}
	// Minimal APP1 EXIF with orientation=6 (rotate 90 CW). Enough for our parser.
	app1 := []byte{
		0xFF, 0xE1, 0x00, 0x1E, // APP1, length (30)
		'E', 'x', 'i', 'f', 0x00, 0x00,
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, // TIFF little endian, IFD at 8
		0x01, 0x00, // 1 entry
		0x12, 0x01, // Orientation tag
		0x03, 0x00, // SHORT
		0x01, 0x00, 0x00, 0x00, // count 1
		0x06, 0x00, 0x00, 0x00, // value 6
		0x00, 0x00, 0x00, 0x00, // next IFD
		0x00, 0x00, // padding
	}
	out := make([]byte, 0, len(b)+len(app1))
	out = append(out, b[:2]...)
	out = append(out, app1...)
	out = append(out, b[2:]...)
	return out
}

func TestDecodeAndProcessStripsMetadataAndSetsDimensions(t *testing.T) {
	// 100x50 with orientation 6 should become 50x100
	data := makeJPEG(t, 100, 50, true)
	got, err := decodeAndProcess(context.Background(), bytes.NewReader(data), 100, "Studio X")
	if err != nil {
		t.Fatalf("decodeAndProcess: %v", err)
	}
	if got.Width != 50 || got.Height != 100 {
		t.Fatalf("dims=%dx%d want 50x100", got.Width, got.Height)
	}
	if bytes.Contains(got.Preview, []byte("Exif")) || bytes.Contains(got.Thumb, []byte("Exif")) {
		t.Fatal("preview/thumb should not contain Exif metadata")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(got.Preview))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width > 1600 || cfg.Height > 1600 {
		t.Fatalf("preview too large: %dx%d", cfg.Width, cfg.Height)
	}
}

func TestDecodeAndProcessRejectsTooLargeMegapixels(t *testing.T) {
	data := makeJPEG(t, 2000, 2000, false) // 4MP
	_, err := decodeAndProcess(context.Background(), bytes.NewReader(data), 1, "Studio")
	if err == nil || !isInvalidImage(err) {
		t.Fatalf("expected invalid image err, got %v", err)
	}
}

func TestDecodeAndProcessRejectsUndecodable(t *testing.T) {
	_, err := decodeAndProcess(context.Background(), bytes.NewReader([]byte("not an image")), 100, "Studio")
	if err == nil || !isInvalidImage(err) {
		t.Fatalf("expected invalid image err, got %v", err)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "photo"},
		{"..", "photo"},
		{"/", "photo"},
		{"../../etc/passwd", "passwd"},
		{"my photo.jpg", "my photo.jpg"},
		{"weird\x00name.png", "weird_name.png"},
	}
	for _, tt := range tests {
		if got := sanitizeFilename(tt.in); got != tt.want {
			t.Fatalf("sanitizeFilename(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestValidateContentType(t *testing.T) {
	for _, ct := range []string{"image/jpeg", "image/png", "image/webp"} {
		if err := validateContentType(ct); err != nil {
			t.Fatalf("%s: %v", ct, err)
		}
	}
	for _, ct := range []string{"image/gif", "application/pdf", "text/plain"} {
		if err := validateContentType(ct); err == nil {
			t.Fatalf("expected error for %s", ct)
		}
	}
}

var _ = png.Encode
