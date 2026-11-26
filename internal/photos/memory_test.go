package photos

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestProcess24MPMemoryBudget(t *testing.T) {
	if os.Getenv("PHOTO_MEMCHECK") != "1" {
		t.Skip("set PHOTO_MEMCHECK=1 to run 24MP memory measurement")
	}
	// 6000x4000 = 24MP
	img := image.NewRGBA(image.Rect(0, 0, 6000, 4000))
	for y := 0; y < 4000; y += 100 {
		for x := 0; x < 6000; x += 100 {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	got, err := decodeAndProcess(context.Background(), bytes.NewReader(data), 100, "Studio Name")
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocDelta := int64(after.TotalAlloc - before.TotalAlloc)
	rssMB := readRSSMB()
	t.Logf("24MP pipeline: in=%dB preview=%dB thumb=%dB dims=%dx%d alloc_delta=%dMB heap_alloc=%dMB heap_sys=%dMB sys=%dMB rss=%dMB", len(data), len(got.Preview), len(got.Thumb), got.Width, got.Height, allocDelta/(1<<20), after.HeapAlloc/(1<<20), after.HeapSys/(1<<20), after.Sys/(1<<20), rssMB)
	// Gate: prefer RSS/heap evidence; Sys can include retained runtime arenas.
	if rssMB > 350 {
		t.Fatalf("memory RSS=%dMB exceeds 350MB budget", rssMB)
	}
}

func readRSSMB() int64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb / 1024
			}
		}
	}
	return -1
}
