package automation

import (
	"math"
	"testing"
)

func TestBitmapSimilarityRejectsNaNFraction(t *testing.T) {
	bitmap, err := NewBitmap(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if BitmapsSimilar(bitmap, bitmap, 0, math.NaN()) {
		t.Fatal("NaN fraction authorized an identical frame")
	}
}
