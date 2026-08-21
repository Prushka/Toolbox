package automation

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"sync"
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
	if _, ok := SearchPixel(nil, Point{}, Point{}, RGB{}, Tolerance(0)); ok {
		t.Fatal("nil bitmap unexpectedly matched")
	}
	if err := bad.WritePNG(&bytes.Buffer{}); err == nil {
		t.Fatal("expected malformed bitmap PNG error")
	}
	malformed := &Bitmap{Width: maxInt, Height: 1, Pixels: make([]byte, 4)}
	_ = malformed.RGBAt(maxInt-1, 0)
	malformed.Set(maxInt-1, 0, RGB{1, 2, 3})
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
	if _, path, e = ParseImageSearchOptions(`*w10 "C:\\Program Files\\icon.png"`); e != nil || path != `C:\\Program Files\\icon.png` {
		t.Fatalf("quoted path=%q,%v", path, e)
	}
}

type solidImage struct {
	bounds image.Rectangle
	c      color.Color
}

func (s solidImage) ColorModel() color.Model { return color.RGBAModel }
func (s solidImage) Bounds() image.Rectangle { return s.bounds }
func (s solidImage) At(x, y int) color.Color { return s.c }

func TestTemplateOwnershipAndLimits(t *testing.T) {
	source, _ := NewBitmap(2, 1)
	source.Set(0, 0, RGB{1, 2, 3})
	source.Set(1, 0, RGB{9, 9, 9})
	transparent := RGB{1, 2, 3}
	tplImage := image.NewRGBA(image.Rect(0, 0, 2, 1))
	tplImage.SetRGBA(0, 0, color.RGBA{1, 2, 3, 255})
	tplImage.SetRGBA(1, 0, color.RGBA{9, 9, 9, 255})
	tpl, err := CompileTemplate(tplImage, ImageSearchOptions{Transparent: &transparent})
	if err != nil {
		t.Fatal(err)
	}
	transparent = RGB{99, 99, 99}
	if _, ok, err := SearchTemplate(source, Rect{}, tpl); err != nil || !ok {
		t.Fatalf("template changed after option mutation: %v,%v", ok, err)
	}
	tooLarge := solidImage{bounds: image.Rect(0, 0, 1<<30, 1), c: color.RGBA{255, 255, 255, 255}}
	if _, err := CompileTemplate(tooLarge, ImageSearchOptions{}); err == nil {
		t.Fatal("expected huge template rejection")
	}
	bad := &Template{bitmap: templateBitmap{Width: 2, Height: 2, Pixels: make([]byte, 1), Alpha: make([]uint8, 1)}}
	if _, _, err := SearchTemplate(source, Rect{}, bad); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("malformed template error=%v", err)
	}
	badAnchor := &Template{bitmap: templateBitmap{Width: 1, Height: 1, Pixels: make([]byte, 3), Alpha: make([]uint8, 1), Anchors: []int{1}}}
	if _, _, err := SearchTemplate(source, Rect{}, badAnchor); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("malformed anchor error=%v", err)
	}
	if _, err := CompileTemplate(tplImage, ImageSearchOptions{Width: -1, Height: -1}); !errors.Is(err, ErrInvalidRect) {
		t.Fatalf("ambiguous scale error=%v", err)
	}
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
	if e := Sleep(nil, time.Millisecond); !errors.Is(e, ErrInvalidArgument) {
		t.Fatalf("nil Sleep context=%v", e)
	}
	if e := PreciseSleep(nil, time.Millisecond); !errors.Is(e, ErrInvalidArgument) {
		t.Fatalf("nil PreciseSleep context=%v", e)
	}
	if e := JitterSleep(nil, time.Millisecond, 0, 0, r); !errors.Is(e, ErrInvalidArgument) {
		t.Fatalf("nil JitterSleep context=%v", e)
	}
	if e := WaitUntil(nil, time.Millisecond, func() (bool, error) { return true, nil }); !errors.Is(e, ErrInvalidArgument) {
		t.Fatalf("nil WaitUntil context=%v", e)
	}
	if e := WaitUntil(context.Background(), time.Millisecond, nil); !errors.Is(e, ErrInvalidArgument) {
		t.Fatalf("nil WaitUntil predicate=%v", e)
	}
	if e := JitterSleep(context.Background(), time.Duration(1<<63-1), 0, 1, r); !errors.Is(e, ErrInvalidArgument) {
		t.Fatalf("duration overflow=%v", e)
	}
}

func TestConcurrentJitterAndTimerClose(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = JitterSleep(context.Background(), 0, time.Millisecond, 0, r)
		}()
	}
	wg.Wait()
	timer, err := BeginTimerResolution(1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = timer.Close() }()
	}
	wg.Wait()
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
