package automation

import (
	"math"
	"testing"
)

func BenchmarkColorRangeCount(b *testing.B) {
	bitmap, err := NewBitmap(320, 180)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < len(bitmap.Pixels); i += 4 {
		bitmap.Pixels[i], bitmap.Pixels[i+1], bitmap.Pixels[i+2] = byte(i/4), byte(i/97), byte(i/373)
	}
	green := ColorRange{HueMin: 85, HueMax: 150, SatMin: .45, ValMin: .4}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		bitmap.Count(Rect{}, green.Matches)
	}
}

func TestHSVRoundTripColorCube(t *testing.T) {
	for r := 0; r <= 255; r += 17 {
		for g := 0; g <= 255; g += 17 {
			for blue := 0; blue <= 255; blue += 17 {
				color := RGB{uint8(r), uint8(g), uint8(blue)}
				hsv := color.HSV()
				if !(hsv.H >= 0 && hsv.H < 360 && hsv.S >= 0 && hsv.S <= 1 && hsv.V >= 0 && hsv.V <= 1) {
					t.Fatalf("%v yielded invalid HSV %v", color, hsv)
				}
				chroma := hsv.V * hsv.S
				x := chroma * (1 - math.Abs(math.Mod(hsv.H/60, 2)-1))
				m := hsv.V - chroma
				var channels [3]float64
				switch int(hsv.H / 60) {
				case 0:
					channels = [3]float64{chroma, x, 0}
				case 1:
					channels = [3]float64{x, chroma, 0}
				case 2:
					channels = [3]float64{0, chroma, x}
				case 3:
					channels = [3]float64{0, x, chroma}
				case 4:
					channels = [3]float64{x, 0, chroma}
				case 5:
					channels = [3]float64{chroma, 0, x}
				}
				for index, want := range [3]int{r, g, blue} {
					if got := (channels[index] + m) * 255; math.Abs(got-float64(want)) > 1e-10 {
						t.Fatalf("%v HSV round trip channel %d = %v, want %d", color, index, got, want)
					}
				}
			}
		}
	}
}

func TestColorRangeRejectsNaNBoundsAndComponents(t *testing.T) {
	green := ColorRange{HueMin: 85, HueMax: 150, SatMin: .45, ValMin: .4}
	for _, hsv := range []HSV{{H: 100, S: math.NaN(), V: 1}, {H: 100, S: 1, V: math.NaN()}} {
		if green.MatchesHSV(hsv) {
			t.Errorf("accepted invalid color %v", hsv)
		}
	}
	green.SatMin = math.NaN()
	if green.Matches(RGB{0, 255, 0}) {
		t.Fatal("accepted NaN saturation bound")
	}
}
