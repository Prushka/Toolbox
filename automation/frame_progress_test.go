package automation

import (
	"testing"
	"time"
)

func TestFrameProgressRequiresContinuousObservations(t *testing.T) {
	frame := &Bitmap{Width: 1, Height: 1, Pixels: []byte{1, 2, 3, 255}}
	p := FrameProgress{MaxGap: 3 * time.Second}
	now := time.Now()
	for i := 0; i <= 601; i++ {
		duration, changed := p.Observe(now.Add(time.Duration(i)*time.Second), frame)
		if duration != time.Duration(i)*time.Second || changed {
			t.Fatalf("sample %d: %v changed=%t", i, duration, changed)
		}
	}
	changedFrame := &Bitmap{Width: 1, Height: 1, Pixels: []byte{9, 2, 3, 255}}
	if duration, changed := p.Observe(now.Add(602*time.Second), changedFrame); duration != 0 || !changed {
		t.Fatal("changed frame did not reset proof")
	}
	if duration, _ := p.Observe(now.Add(610*time.Second), changedFrame); duration != 0 {
		t.Fatal("observation gap counted as freeze")
	}
	p.Observe(now.Add(611*time.Second), nil)
	if duration, _ := p.Observe(now.Add(612*time.Second), changedFrame); duration != 0 {
		t.Fatal("invalid capture counted as freeze")
	}
	if duration, _ := p.Observe(now.Add(611*time.Second), changedFrame); duration != 0 {
		t.Fatal("backward timestamp counted as freeze")
	}
}
