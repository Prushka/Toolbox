package automation

// clipRegion normalizes region to the bitmap and reports whether anything
// remains. A zero region means the whole bitmap.
func (b *Bitmap) clipRegion(region Rect) (Rect, bool) {
	if !b.valid() {
		return Rect{}, false
	}
	if region == (Rect{}) {
		return Rect{0, 0, b.Width, b.Height}, true
	}
	region = region.Normalize()
	if region.Left < 0 {
		region.Left = 0
	}
	if region.Top < 0 {
		region.Top = 0
	}
	if region.Right > b.Width {
		region.Right = b.Width
	}
	if region.Bottom > b.Height {
		region.Bottom = b.Height
	}
	return region, !region.Empty()
}

// Count returns how many pixels inside region satisfy predicate, and how many
// pixels the clipped region contains. Both are zero for an empty region.
func (b *Bitmap) Count(region Rect, predicate func(RGB) bool) (matched, total int) {
	region, ok := b.clipRegion(region)
	if !ok || predicate == nil {
		return 0, 0
	}
	for y := region.Top; y < region.Bottom; y++ {
		i := (y*b.Width + region.Left) * 4
		for x := region.Left; x < region.Right; x++ {
			if predicate(RGB{b.Pixels[i], b.Pixels[i+1], b.Pixels[i+2]}) {
				matched++
			}
			i += 4
		}
	}
	return matched, region.Width() * region.Height()
}

// Fraction returns the share of pixels inside region that satisfy predicate,
// in [0, 1]. An empty region yields zero.
func (b *Bitmap) Fraction(region Rect, predicate func(RGB) bool) float64 {
	matched, total := b.Count(region, predicate)
	if total == 0 {
		return 0
	}
	return float64(matched) / float64(total)
}

// Mean returns the average color inside region.
func (b *Bitmap) Mean(region Rect) RGB {
	region, ok := b.clipRegion(region)
	if !ok {
		return RGB{}
	}
	// A supported large bitmap can exceed a 32-bit channel accumulator.
	var r, g, bl, n uint64
	for y := region.Top; y < region.Bottom; y++ {
		i := (y*b.Width + region.Left) * 4
		for x := region.Left; x < region.Right; x++ {
			r += uint64(b.Pixels[i])
			g += uint64(b.Pixels[i+1])
			bl += uint64(b.Pixels[i+2])
			n++
			i += 4
		}
	}
	if n == 0 {
		return RGB{}
	}
	return RGB{uint8(r / n), uint8(g / n), uint8(bl / n)}
}

// ColorMatcher returns a predicate for an RGB color with per-channel tolerance.
func ColorMatcher(want RGB, tolerance ColorTolerance) func(RGB) bool {
	return func(c RGB) bool { return c.Matches(want, tolerance) }
}

// DarkerThan returns a predicate that accepts pixels whose every channel is
// below the limit.
func DarkerThan(limit uint8) func(RGB) bool {
	return func(c RGB) bool { return c.R < limit && c.G < limit && c.B < limit }
}
