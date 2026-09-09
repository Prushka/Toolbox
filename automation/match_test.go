package automation

import (
	"errors"
	"image"
	"image/color"
	"math"
	"sync"
	"testing"
)

func TestCorrelationRejectsSpatiallyFlatColors(t *testing.T) {
	t.Parallel()
	flat, _ := NewBitmap(20, 20)
	for y := 0; y < flat.Height; y++ {
		for x := 0; x < flat.Width; x++ {
			flat.Set(x, y, RGB{30, 150, 220})
		}
	}
	template, err := CompileTemplate(flat, ImageSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if template.HasContrast() {
		t.Error("a flat blue template has no spatial contrast")
	}
	pattern, _ := CompileTemplate(gradientImage(20, 20), ImageSearchOptions{})
	score, err := TemplateScoreAt(flat, Point{}, pattern)
	if err != nil || score != 0 {
		t.Errorf("flat source score = %v, %v; want zero", score, err)
	}
}

func TestCorrelationValidatesScoreAndOverflowingLocation(t *testing.T) {
	t.Parallel()
	source, _ := NewBitmap(40, 40)
	template, _ := CompileTemplate(gradientImage(20, 20), ImageSearchOptions{})
	if _, _, err := SearchTemplateScore(source, Rect{}, template, TemplateMatchOptions{MinScore: math.NaN()}); !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("NaN threshold error = %v", err)
	}
	if _, err := TemplateScoreAt(source, Point{maxInt, maxInt}, template); !errors.Is(err, ErrInvalidRect) {
		t.Errorf("overflowing location error = %v", err)
	}
}

func TestCorrelationLowContrastExactScore(t *testing.T) {
	t.Parallel()
	source, _ := NewBitmap(300, 300)
	for y := 0; y < source.Height; y++ {
		for x := 0; x < source.Width; x++ {
			v := uint8(240 + (x+y)%2)
			source.Set(x, y, RGB{v, v, v})
		}
	}
	template, _ := CompileTemplate(source, ImageSearchOptions{})
	score, err := TemplateScoreAt(source, Point{}, template)
	if err != nil || score < 0.99999 || score > 1 {
		t.Fatalf("low contrast exact score = %v, %v; want 1", score, err)
	}
}

func TestCorrelationExhaustiveFindsNarrowPeak(t *testing.T) {
	t.Parallel()
	pattern, _ := NewBitmap(40, 40)
	var state uint32 = 123456789
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			state = state*1664525 + 1013904223
			pattern.Set(x, y, RGB{uint8(state >> 24), uint8(state >> 16), uint8(state >> 8)})
		}
	}
	template, _ := CompileTemplate(pattern, ImageSearchOptions{})
	source, _ := NewBitmap(49, 49)
	for y := 0; y < 40; y++ {
		for x := 0; x < 40; x++ {
			source.Set(x+9, y+9, pattern.RGBAt(x, y))
		}
	}
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			// Exercise the shared lazy compilation on first use, too.
			match, found, err := SearchTemplateScore(source, Rect{}, template, TemplateMatchOptions{Exhaustive: true, MinScore: 0.99})
			if err != nil || !found || match.Point != (Point{9, 9}) {
				t.Errorf("exhaustive edge match = %+v, found=%v, error=%v", match, found, err)
			}
		})
	}
	workers.Wait()
}

func BenchmarkTemplateScoreRegion(b *testing.B) {
	source, _ := NewBitmap(320, 180)
	pattern := gradientImage(48, 48)
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			c := pattern.NRGBAAt(x, y)
			source.Set(x+101, y+51, RGB{c.R, c.G, c.B})
		}
	}
	template, _ := CompileTemplate(pattern, ImageSearchOptions{})
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, found, err := SearchTemplateScore(source, Rect{}, template, TemplateMatchOptions{})
		if err != nil || !found {
			b.Fatalf("match = %v, %v", found, err)
		}
	}
}

func BenchmarkCompileToleranceTemplate(b *testing.B) {
	pattern := gradientImage(128, 128)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := CompileTemplate(pattern, ImageSearchOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func TestHSVConversionAndColorRange(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		color RGB
		want  HSV
	}{
		{RGB{255, 0, 0}, HSV{0, 1, 1}},
		{RGB{0, 255, 0}, HSV{120, 1, 1}},
		{RGB{0, 0, 255}, HSV{240, 1, 1}},
		{RGB{255, 255, 255}, HSV{0, 0, 1}},
		{RGB{0, 0, 0}, HSV{0, 0, 0}},
		{RGB{128, 0, 128}, HSV{300, 1, 128.0 / 255}},
	} {
		got := test.color.HSV()
		if math.Abs(got.H-test.want.H) > 0.01 || math.Abs(got.S-test.want.S) > 0.01 || math.Abs(got.V-test.want.V) > 0.01 {
			t.Errorf("%v.HSV() = %+v, want %+v", test.color, got, test.want)
		}
	}
	lime := ColorRange{HueMin: 70, HueMax: 110, SatMin: 0.6, ValMin: 0.5}
	if !lime.Matches(RGB{0x9C, 0xFF, 0x00}) {
		t.Fatal("lime range rejected the reference lime")
	}
	// A dimmer, less saturated rendition of the same hue still matches.
	if !lime.Matches(RGB{0x70, 0xC0, 0x30}) {
		t.Fatal("lime range rejected a tone-mapped lime")
	}
	if lime.Matches(RGB{0xFF, 0x00, 0x00}) || lime.Matches(RGB{0x30, 0x40, 0x30}) {
		t.Fatal("lime range accepted red or dark grey")
	}
	red := ColorRange{HueMin: 340, HueMax: 20, SatMin: 0.5, ValMin: 0.3}
	if !red.Matches(RGB{255, 20, 30}) || !red.Matches(RGB{255, 30, 60}) {
		t.Fatal("wrapped hue range rejected red")
	}
	if red.Matches(RGB{255, 200, 0}) {
		t.Fatal("wrapped hue range accepted yellow")
	}
	if (ColorRange{SatMin: 0.5}).Matches(RGB{200, 200, 200}) {
		t.Fatal("saturation floor accepted grey")
	}
	if !(ColorRange{ValMax: 0.2}).Matches(RGB{20, 30, 40}) {
		t.Fatal("value ceiling rejected a dark pixel")
	}
	if got := (RGB{255, 255, 255}).Luma(); got != 255 {
		t.Fatalf("white luma = %d", got)
	}
}

func TestBitmapCountFractionAndMean(t *testing.T) {
	t.Parallel()
	b, err := NewBitmap(4, 2)
	if err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 4; x++ {
		b.Set(x, 0, RGB{255, 0, 0})
		b.Set(x, 1, RGB{0, 0, 255})
	}
	matched, total := b.Count(Rect{}, ColorMatcher(RGB{255, 0, 0}, Tolerance(0)))
	if matched != 4 || total != 8 {
		t.Fatalf("Count = %d/%d, want 4/8", matched, total)
	}
	if got := b.Fraction(Rect{0, 1, 4, 2}, ColorMatcher(RGB{0, 0, 255}, Tolerance(0))); got != 1 {
		t.Fatalf("Fraction(bottom row) = %v, want 1", got)
	}
	if got := b.Fraction(Rect{-5, -5, 100, 100}, DarkerThan(10)); got != 0 {
		t.Fatalf("Fraction(clipped) = %v, want 0", got)
	}
	if got := b.Fraction(Rect{10, 10, 20, 20}, DarkerThan(10)); got != 0 {
		t.Fatalf("Fraction(outside) = %v, want 0", got)
	}
	if got := b.Mean(Rect{}); got != (RGB{127, 0, 127}) {
		t.Fatalf("Mean = %v, want {127 0 127}", got)
	}
	var nilBitmap *Bitmap
	if matched, total := nilBitmap.Count(Rect{}, DarkerThan(1)); matched != 0 || total != 0 {
		t.Fatal("nil bitmap counted pixels")
	}
}

// gradientImage draws a distinctive pattern so correlation has contrast.
func gradientImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{uint8(x * 255 / width), uint8(y * 255 / height), uint8((x * y) % 256), 255})
		}
	}
	return img
}

func TestSearchTemplateScoreFindsToneMappedTemplate(t *testing.T) {
	t.Parallel()
	source, err := NewBitmap(120, 90)
	if err != nil {
		t.Fatal(err)
	}
	// Busy background so unrelated windows do not correlate.
	for y := 0; y < source.Height; y++ {
		for x := 0; x < source.Width; x++ {
			source.Set(x, y, RGB{uint8((x*7 + y*13) % 256), uint8((x * y) % 256), uint8((x + y*3) % 256)})
		}
	}
	pattern := gradientImage(20, 14)
	// Paste a dimmed, lower-contrast rendition of the pattern at (37, 51).
	for y := 0; y < 14; y++ {
		for x := 0; x < 20; x++ {
			c := pattern.NRGBAAt(x, y)
			source.Set(37+x, 51+y, RGB{uint8(20 + int(c.R)*7/10), uint8(20 + int(c.G)*7/10), uint8(20 + int(c.B)*7/10)})
		}
	}
	template, err := CompileTemplate(pattern, ImageSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !template.HasContrast() {
		t.Fatal("gradient template reported no contrast")
	}
	if _, found, err := SearchTemplate(source, Rect{}, template); err != nil || found {
		t.Fatalf("tolerance search found a dimmed template: found=%v err=%v", found, err)
	}
	for _, stride := range []int{1, 2, 3} {
		match, found, err := SearchTemplateScore(source, Rect{}, template, TemplateMatchOptions{MinScore: 0.9, Stride: stride})
		if err != nil {
			t.Fatal(err)
		}
		if !found || match.Point != (Point{37, 51}) {
			t.Fatalf("stride %d: match = %+v found=%v, want (37,51)", stride, match, found)
		}
		if match.Score < 0.99 {
			t.Fatalf("stride %d: score = %v, want near 1", stride, match.Score)
		}
	}
	score, err := TemplateScoreAt(source, Point{37, 51}, template)
	if err != nil || score < 0.99 {
		t.Fatalf("TemplateScoreAt = %v, %v", score, err)
	}
	if _, err := TemplateScoreAt(source, Point{110, 80}, template); err == nil {
		t.Fatal("TemplateScoreAt accepted a window outside the source")
	}
	// A region that excludes the pattern must miss.
	if _, found, err := SearchTemplateScore(source, Rect{0, 0, 40, 40}, template, TemplateMatchOptions{}); err != nil || found {
		t.Fatalf("region search found a match outside its bounds: found=%v err=%v", found, err)
	}
	if _, _, err := SearchTemplateScore(source, Rect{}, template, TemplateMatchOptions{MinScore: 2}); err == nil {
		t.Fatal("invalid MinScore was accepted")
	}
}

func TestSearchTemplateScoreRejectsFlatTemplate(t *testing.T) {
	t.Parallel()
	flat := image.NewNRGBA(image.Rect(0, 0, 5, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			flat.Set(x, y, color.NRGBA{10, 10, 10, 255})
		}
	}
	template, err := CompileTemplate(flat, ImageSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if template.HasContrast() {
		t.Fatal("flat template reported contrast")
	}
	source, _ := NewBitmap(10, 10)
	if _, found, err := SearchTemplateScore(source, Rect{}, template, TemplateMatchOptions{}); err != nil || found {
		t.Fatalf("flat template matched: found=%v err=%v", found, err)
	}
}

func TestSearchTemplateScoreHonorsTransparency(t *testing.T) {
	t.Parallel()
	pattern := gradientImage(16, 16)
	// Make the left half wildcard; only the right half must correlate.
	for y := 0; y < 16; y++ {
		for x := 0; x < 8; x++ {
			pattern.Set(x, y, color.NRGBA{0, 0, 0, 0})
		}
	}
	template, err := CompileTemplate(pattern, ImageSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	source, _ := NewBitmap(40, 40)
	for y := 0; y < 16; y++ {
		for x := 8; x < 16; x++ {
			c := pattern.NRGBAAt(x, y)
			source.Set(12+x, 9+y, RGB{c.R, c.G, c.B})
		}
	}
	// Garbage under the wildcard half must not matter.
	for y := 0; y < 16; y++ {
		for x := 0; x < 8; x++ {
			source.Set(12+x, 9+y, RGB{uint8(x * 30), 200, uint8(y * 15)})
		}
	}
	match, found, err := SearchTemplateScore(source, Rect{}, template, TemplateMatchOptions{MinScore: 0.95})
	if err != nil || !found || match.Point != (Point{12, 9}) {
		t.Fatalf("transparent template match = %+v found=%v err=%v", match, found, err)
	}
}
