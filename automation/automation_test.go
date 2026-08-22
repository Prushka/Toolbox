package automation

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
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
	r, g, bl, a := img.At(1, 1).RGBA()
	if uint8(r>>8) != 10 || uint8(g>>8) != 20 || uint8(bl>>8) != 30 {
		t.Fatalf("decoded color %d,%d,%d", r>>8, g>>8, bl>>8)
	}
	if a != 0xffff {
		t.Fatalf("decoded alpha=%d, want opaque", a)
	}
	_, _, _, a = img.At(0, 0).RGBA()
	if a != 0xffff {
		t.Fatalf("unset pixel alpha=%d, want opaque", a)
	}

	// Raw alpha bytes are padding and must not change opaque PNG semantics.
	b.Pixels[3] = 1
	buf.Reset()
	if err := b.WritePNG(&buf); err != nil {
		t.Fatal(err)
	}
	img, err = png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, a = img.At(0, 0).RGBA()
	if a != 0xffff {
		t.Fatalf("raw-alpha pixel=%d, want opaque", a)
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
	for _, region := range []Rect{
		{-4, 0, -1, 3},
		{4, 0, 8, 3},
		{0, -4, 4, -1},
		{0, 3, 4, 8},
	} {
		if p, ok := SearchPixelRect(b, region, RGB{100, 100, 100}, Tolerance(0)); ok {
			t.Errorf("out-of-bounds region %+v matched at %v", region, p)
		}
	}
}

func TestBitmapRejectsInvalidStorage(t *testing.T) {
	if _, e := NewBitmap(int(^uint(0)>>1), 2); e == nil {
		t.Fatal("expected overflow guard")
	}
	bad := &Bitmap{Width: 2, Height: 2, Pixels: make([]byte, 1)}
	if clone := bad.Clone(); clone != nil {
		t.Fatalf("malformed clone=%+v, want nil", clone)
	}
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
	overflowingIndex := &Bitmap{Width: maxInt/2 + 1, Height: 3, Pixels: make([]byte, 4)}
	_ = overflowingIndex.RGBAt(maxInt/2, 1)
	overflowingIndex.Set(maxInt/2, 1, RGB{1, 2, 3})
	withTrailingStorage := &Bitmap{Width: 1, Height: 1, Pixels: []byte{1, 2, 3, 255, 4, 5, 6, 7}}
	clone := withTrailingStorage.Clone()
	if clone == nil || len(clone.Pixels) != 4 || clone.RGBAt(0, 0) != (RGB{1, 2, 3}) {
		t.Fatalf("clone with trailing storage=%+v", clone)
	}
}

func TestSearchOriginNormalizesAndClips(t *testing.T) {
	tests := []struct {
		region Rect
		want   Point
	}{
		{Rect{}, Point{}},
		{Rect{10, 20, 30, 40}, Point{10, 20}},
		{Rect{30, 40, 10, 20}, Point{10, 20}},
		{Rect{-30, -40, 10, 20}, Point{}},
		{Rect{10, 20, 10, 40}, Point{}},
	}
	for _, test := range tests {
		if got := searchOrigin(test.region); got != test.want {
			t.Errorf("searchOrigin(%+v)=%+v, want %+v", test.region, got, test.want)
		}
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

	transparent := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	transparent.SetNRGBA(0, 0, color.NRGBA{200, 100, 50, 0})
	if p, ok, e := SearchImage(s, Rect{1, 1, 6, 5}, transparent, ImageSearchOptions{}); e != nil || !ok || p != (Point{1, 1}) {
		t.Fatalf("fully transparent template=%v,%v,%v", p, ok, e)
	}

	partial := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	partial.SetNRGBA(0, 0, color.NRGBA{20, 30, 40, 128})
	if p, ok, e := SearchImage(s, Rect{}, partial, ImageSearchOptions{}); e != nil || !ok || p != (Point{2, 1}) {
		t.Fatalf("partially transparent RGB=%v,%v,%v", p, ok, e)
	}
}

func TestSearchTemplateClipsRegions(t *testing.T) {
	source, _ := NewBitmap(6, 5)
	templateImage := image.NewRGBA(image.Rect(0, 0, 2, 2))
	template, err := CompileTemplate(templateImage, ImageSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, region := range []Rect{
		{-4, 0, -1, 5},
		{6, 0, 10, 5},
		{0, -4, 6, -1},
		{0, 5, 6, 10},
		{0, 0, 1, 1},
	} {
		if p, ok, err := SearchTemplate(source, region, template); err != nil || ok {
			t.Errorf("region %+v returned %v,%v,%v; want miss", region, p, ok, err)
		}
	}
}

func TestImageLoadersRejectOversizedHeader(t *testing.T) {
	var header [33]byte
	copy(header[:8], "\x89PNG\r\n\x1a\n")
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], "IHDR")
	binary.BigEndian.PutUint32(header[16:20], 1<<20)
	binary.BigEndian.PutUint32(header[20:24], 1<<20)
	header[24], header[25] = 8, 2
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	path := filepath.Join(t.TempDir(), "oversized.png")
	if err := os.WriteFile(path, header[:], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTemplate(path, ImageSearchOptions{}); err == nil {
		t.Fatal("oversized template header was accepted")
	}
	if _, err := LoadImage(path); err == nil {
		t.Fatal("oversized image header was accepted")
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
	if _, path, e = ParseImageSearchOptions("\t*w10  C:\\Program Files\\my  icon.png  "); e != nil || path != `C:\Program Files\my  icon.png` {
		t.Fatalf("space-preserving path=%q,%v", path, e)
	}
	if _, path, e = ParseImageSearchOptions("*w10 \"C:\\Program Files\\my  icon.png\""); e != nil || path != `C:\Program Files\my  icon.png` {
		t.Fatalf("quoted space-preserving path=%q,%v", path, e)
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
	immediateCalls := 0
	if e := WaitUntil(context.Background(), time.Hour, func() (bool, error) {
		immediateCalls++
		return true, nil
	}); e != nil || immediateCalls != 1 {
		t.Fatalf("immediate WaitUntil=%v, calls=%d", e, immediateCalls)
	}
	if e := JitterSleep(context.Background(), time.Duration(1<<63-1), 0, 1, r); !errors.Is(e, ErrInvalidArgument) {
		t.Fatalf("duration overflow=%v", e)
	}
}

type maxRandSource struct{}

func (maxRandSource) Int63() int64   { return 1<<63 - 1 }
func (maxRandSource) Uint64() uint64 { return ^uint64(0) }
func (maxRandSource) Seed(int64)     {}

func TestJitterDurationBounds(t *testing.T) {
	maxDuration := time.Duration(1<<63 - 1)
	tests := []struct {
		name           string
		d, minus, plus time.Duration
		lower, upper   time.Duration
	}{
		{"negative base crossing zero", -10 * time.Millisecond, 20 * time.Millisecond, 15 * time.Millisecond, 0, 5 * time.Millisecond},
		{"negative range", -10 * time.Millisecond, 0, 5 * time.Millisecond, 0, 0},
		{"minus exceeds base", 10 * time.Millisecond, 20 * time.Millisecond, 3 * time.Millisecond, 0, 13 * time.Millisecond},
		{"negative jitter values clamp", 10 * time.Millisecond, -time.Millisecond, -time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond},
		{"near maximum", maxDuration - 5, 2, 5, maxDuration - 7, maxDuration},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(3))
			for i := 0; i < 128; i++ {
				got, err := jitterDuration(test.d, test.minus, test.plus, r)
				if err != nil {
					t.Fatal(err)
				}
				if got < test.lower || got > test.upper {
					t.Fatalf("duration %v outside [%v,%v]", got, test.lower, test.upper)
				}
			}
		})
	}
	if _, err := jitterDuration(maxDuration, 0, 1, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("duration overflow=%v", err)
	}
	r := rand.New(maxRandSource{})
	if got, err := jitterDuration(0, 0, maxDuration, r); err != nil || got != maxDuration {
		t.Fatalf("full-range upper endpoint=%v,%v", got, err)
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

func BenchmarkSearchPixel1080p(b *testing.B) {
	source, _ := NewBitmap(1920, 1080)
	want := RGB{255, 255, 255}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SearchPixelRect(source, Rect{}, want, Tolerance(0))
	}
}
