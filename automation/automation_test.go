package automation

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBitmapAndPNG(t *testing.T) {
	b, err := NewBitmap(3, 2)
	if err != nil {
		t.Fatal(err)
	}
	b.Set(1, 1, RGB{10, 20, 30})
	if got := b.RGBAt(1, 1); got != (RGB{10, 20, 30}) {
		t.Fatalf("RGBAt=%v", got)
	}
	var buf bytes.Buffer
	if err := b.WritePNG(&buf); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	r, g, bl, _ := img.At(1, 1).RGBA()
	if uint8(r>>8) != 10 || uint8(g>>8) != 20 || uint8(bl>>8) != 30 {
		t.Fatalf("decoded color %d,%d,%d", r>>8, g>>8, bl>>8)
	}
}

func TestPixelSearchSemantics(t *testing.T) {
	b, _ := NewBitmap(4, 3)
	b.Set(1, 0, RGB{100, 100, 100})
	b.Set(3, 2, RGB{100, 100, 100})
	if p, ok := SearchPixel(b, Point{0, 0}, Point{3, 2}, RGB{100, 100, 100}, Tolerance(0)); !ok || p != (Point{1, 0}) {
		t.Fatalf("first match=%v,%v", p, ok)
	}
	if p, ok := SearchPixel(b, Point{3, 2}, Point{0, 0}, RGB{100, 100, 100}, Tolerance(0)); !ok || p != (Point{3, 2}) {
		t.Fatalf("reverse match=%v,%v", p, ok)
	}
	if _, ok := SearchPixelRect(b, Rect{0, 0, 4, 3}, RGB{105, 100, 100}, ColorTolerance{5, 0, 0}); !ok {
		t.Fatal("tolerance should match")
	}
	if p, ok := SearchPixelRect(b, Rect{4, 3, 0, 0}, RGB{100, 100, 100}, Tolerance(0)); !ok || p != (Point{1, 0}) {
		t.Fatalf("normalized rect=%v,%v", p, ok)
	}
	if p, ok := SearchPixel(b, Point{-1000000, -1000000}, Point{1000000, 1000000}, RGB{100, 100, 100}, Tolerance(0)); !ok || p != (Point{1, 0}) {
		t.Fatalf("clipped search=%v,%v", p, ok)
	}
}

func TestBitmapRejectsInvalidStorage(t *testing.T) {
	if _, e := NewBitmap(int(^uint(0)>>1), 2); e == nil {
		t.Fatal("expected overflow guard")
	}
	bad := &Bitmap{Width: 2, Height: 2, Pixels: make([]byte, 1)}
	if _, e := bad.Crop(Rect{}); e == nil {
		t.Fatal("expected malformed bitmap error")
	}
}

func TestImageSearchVariationTransparencyAndScale(t *testing.T) {
	s, _ := NewBitmap(6, 5)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			s.Set(x+2, y+1, RGB{20 + uint8(x), 30 + uint8(y), 40})
		}
	}
	tpl := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			tpl.SetRGBA(x, y, color.RGBA{uint8(20 + x), uint8(30 + y), 40, 255})
		}
	}
	p, ok, e := SearchImage(s, Rect{}, tpl, ImageSearchOptions{})
	if e != nil || !ok || p != (Point{2, 1}) {
		t.Fatalf("search=%v,%v,%v", p, ok, e)
	}
	tpl.SetRGBA(1, 0, color.RGBA{255, 0, 0, 0})
	if _, ok, e = SearchImage(s, Rect{}, tpl, ImageSearchOptions{}); e != nil || !ok {
		t.Fatalf("alpha wildcard: %v,%v", ok, e)
	}
	if _, ok, e = SearchImage(s, Rect{}, tpl, ImageSearchOptions{Variation: 1}); e != nil || !ok {
		t.Fatalf("variation: %v,%v", ok, e)
	}
}

func TestParseImageOptions(t *testing.T) {
	o, path, e := ParseImageSearchOptions("*42 *TransBlack *w100 *h-1 icon.png")
	if e != nil {
		t.Fatal(e)
	}
	if o.Variation != 42 || o.Transparent == nil || o.Transparent.Uint32() != 0 || o.Width != 100 || o.Height != -1 || path != "icon.png" {
		t.Fatalf("%+v %q", o, path)
	}
	if _, _, e = ParseImageSearchOptions("*300 x"); e == nil {
		t.Fatal("expected range error")
	}
	if _, _, e = ParseImageSearchOptions("*NoSuch x"); e == nil {
		t.Fatal("expected unsupported option error")
	}
}

func TestINIAndLogger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.ini")
	if e := WriteINI(path, "settings", "logging", "true"); e != nil {
		t.Fatal(e)
	}
	if got, e := ReadINI(path, "SETTINGS", "LOGGING", "false"); e != nil || got != "true" {
		t.Fatalf("read=%q,%v", got, e)
	}
	if e := WriteINI(path, "settings", "difficulty", "hard"); e != nil {
		t.Fatal(e)
	}
	if got, e := ReadINI(path, "settings", "logging", "false"); e != nil || got != "true" {
		t.Fatalf("replacement lost prior key: %q,%v", got, e)
	}
	var out bytes.Buffer
	l := NewLogger(&out)
	l.Prefix = "test"
	l.Printf("hello %d", 3)
	if !strings.Contains(out.String(), "test hello 3") {
		t.Fatalf("log=%q", out.String())
	}
	_ = os.Remove(path)
}

func TestWaitSleep(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	calls := 0
	e := WaitUntil(ctx, time.Millisecond, func() (bool, error) { calls++; return false, nil })
	if !errors.Is(e, context.DeadlineExceeded) && !errors.Is(e, context.Canceled) {
		t.Fatalf("wait error=%v", e)
	}
	if calls == 0 {
		t.Fatal("predicate not called")
	}
	if e := Sleep(context.Background(), time.Millisecond); e != nil {
		t.Fatal(e)
	}
	r := rand.New(rand.NewSource(1))
	if e := JitterSleep(context.Background(), time.Millisecond, 0, 0, r); e != nil {
		t.Fatal(e)
	}
}

func TestRelativePoint(t *testing.T) {
	if got := RelativePoint(Point{100, 50}, image.Point{200, 100}, image.Point{400, 300}); got != (Point{200, 150}) {
		t.Fatal(got)
	}
}

func BenchmarkSearchTemplate1080p(b *testing.B) {
	s, _ := NewBitmap(1920, 1080)
	for i := range s.Pixels {
		s.Pixels[i] = byte(i*31 + 17)
	}
	tpl := image.NewRGBA(image.Rect(0, 0, 24, 18))
	for y := 0; y < 18; y++ {
		for x := 0; x < 24; x++ {
			tpl.SetRGBA(x, y, color.RGBA{uint8(x * 7), uint8(y * 11), uint8(x + y), 255})
		}
	}
	tm, _ := CompileTemplate(tpl, ImageSearchOptions{})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = SearchTemplate(s, Rect{}, tm)
	}
}
