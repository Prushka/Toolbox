package automation

import "time"

// FrameProgress tracks continuously observed rendering inactivity. Invalid
// frames, non-increasing timestamps, and gaps longer than MaxGap reset proof;
// missing observations cannot establish that an application was frozen.
// It is owned by one goroutine. Captures must be fresh, immutable bitmaps.
type FrameProgress struct {
	MaxGap         time.Duration
	PixelTolerance uint8
	baseline       *Bitmap
	since, last    time.Time
}

func (p *FrameProgress) Reset() { p.baseline = nil; p.since = time.Time{}; p.last = time.Time{} }

// Observe returns the duration continuously unchanged and whether a previously
// observed valid frame changed. Call Reset when the visible surface is unusable.
func (p *FrameProgress) Observe(now time.Time, frame *Bitmap) (time.Duration, bool) {
	if frame == nil || !frame.valid() || now.IsZero() {
		p.Reset()
		return 0, false
	}
	continuous := p.baseline != nil && now.After(p.last) && p.MaxGap > 0 && now.Sub(p.last) <= p.MaxGap
	changed := continuous && !BitmapsSimilar(p.baseline, frame, p.PixelTolerance, 0)
	if !continuous || changed {
		p.baseline = frame
		p.since = now
	}
	p.last = now
	return now.Sub(p.since), changed
}
