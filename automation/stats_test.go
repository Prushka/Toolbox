package automation

import "testing"

func TestMeanLargeBrightBitmap(t *testing.T) {
	bitmap, err := NewBitmap(4096, 2160)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(bitmap.Pixels); i += 4 {
		bitmap.Pixels[i], bitmap.Pixels[i+1], bitmap.Pixels[i+2] = 255, 254, 253
	}
	if got, want := bitmap.Mean(Rect{}), (RGB{255, 254, 253}); got != want {
		t.Fatalf("large bitmap mean = %v, want %v", got, want)
	}
}
