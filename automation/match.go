package automation

import "math"

// TemplateMatchOptions controls SearchTemplateScore.
type TemplateMatchOptions struct {
	// MinScore is the lowest normalized cross-correlation accepted as a match,
	// in (0, 1]. Zero selects the default of 0.85. Typical clean matches score
	// above 0.95; the same artwork under a different display tone mapping or
	// with anti-aliasing differences usually stays above 0.85, while unrelated
	// content rarely exceeds 0.6.
	MinScore float64
	// Stride is the coarse scan step. Zero chooses 1 for small templates and 2
	// for larger ones. Sampled coarse scanning is approximate and can miss
	// narrow peaks or artwork with unrepresentative sampled pixels.
	Stride int
	// Exhaustive scores every possible location without sampling. It guarantees
	// the best score inside the region, at a higher cost for large regions.
	Exhaustive bool
}

// TemplateMatch is a scored template location.
type TemplateMatch struct {
	Point Point
	Score float64
}

const defaultTemplateMinScore = 0.85

// templateCorrelation is precomputed normalized cross-correlation state for a
// compiled template. Only opaque template pixels take part.
type templateCorrelation struct {
	pixels  []int32   // template pixel index (y*width + x) of every opaque pixel
	values  []float64 // channel-centered template values, three per opaque pixel
	norm    float64   // Euclidean norm of values
	samples []int32   // subset of pixels used by the coarse scan
	sample  []float64 // channel-centered values of the sampled subset
	sNorm   float64
}

func compileCorrelation(t templateBitmap) templateCorrelation {
	pixelCount := t.Width * t.Height
	var opaque []int32
	for i := 0; i < pixelCount; i++ {
		if t.Alpha[i] != 0 {
			opaque = append(opaque, int32(i))
		}
	}
	c := templateCorrelation{pixels: opaque}
	if len(opaque) == 0 {
		return c
	}
	c.values, c.norm = zeroMeanChannels(t, opaque)

	// The coarse scan uses an evenly spread subset of the opaque pixels.
	const maxSamples = 48
	step := max((len(opaque)+maxSamples-1)/maxSamples, 1)
	for i := 0; i < len(opaque); i += step {
		c.samples = append(c.samples, opaque[i])
	}
	c.sample, c.sNorm = zeroMeanChannels(t, c.samples)
	if c.sNorm == 0 {
		// A periodic or sparse template can have flat samples despite having
		// full contrast. Do not discard all locations in that case.
		c.samples, c.sample, c.sNorm = c.pixels, c.values, c.norm
	}
	return c
}

func zeroMeanChannels(t templateBitmap, pixels []int32) ([]float64, float64) {
	values := make([]float64, 0, len(pixels)*3)
	var sums [3]float64
	for _, index := range pixels {
		for ch := 0; ch < 3; ch++ {
			v := float64(t.Pixels[int(index)*3+ch])
			values = append(values, v)
			sums[ch] += v
		}
	}
	for ch := range sums {
		sums[ch] /= float64(len(pixels))
	}
	var norm float64
	for i := range values {
		values[i] -= sums[i%3]
		norm += values[i] * values[i]
	}
	return values, math.Sqrt(norm)
}

// HasContrast reports whether the template can be scored by correlation. A
// template that is one flat color has no variance and cannot be located this
// way; use SearchTemplate with a tolerance instead.
func (t *Template) HasContrast() bool {
	return t != nil && t.correlationState().norm > 0
}

// Correlation is compiled only when requested, so tolerance-only consumers do
// not pay for its memory and preprocessing. Compiled templates are shared safely.
func (t *Template) correlationState() *templateCorrelation {
	t.correlationOnce.Do(func() { t.correlation = compileCorrelation(t.bitmap) })
	return &t.correlation
}

// TemplateScoreAt returns the normalized cross-correlation between the
// template and the source window whose top-left corner is at. The score is
// in [-1, 1]; 1 is a perfect match up to brightness and contrast scaling.
func TemplateScoreAt(source *Bitmap, at Point, template *Template) (float64, error) {
	if source == nil || template == nil || !source.valid() || !template.bitmap.valid() {
		return 0, ErrInvalidArgument
	}
	t := template.bitmap
	if at.X < 0 || at.Y < 0 || at.X > source.Width-t.Width || at.Y > source.Height-t.Height {
		return 0, ErrInvalidRect
	}
	c := template.correlationState()
	if c.norm == 0 {
		return 0, nil
	}
	return correlationAt(source, at.X, at.Y, t.Width, c.pixels, c.values, c.norm), nil
}

// sourceOffsets converts template pixel indices to byte offsets inside a
// source bitmap of the given stride, relative to a window's top-left pixel.
func sourceOffsets(pixels []int32, templateWidth, sourceWidth int) []int32 {
	offsets := make([]int32, len(pixels))
	for i, index := range pixels {
		ty := int(index) / templateWidth
		tx := int(index) - ty*templateWidth
		offsets[i] = int32((ty*sourceWidth + tx) * 4)
	}
	return offsets
}

// correlationAt computes NCC over the listed template pixels for the source
// window at (x, y). values holds zero-mean template channels in the same order.
func correlationAt(source *Bitmap, x, y, templateWidth int, pixels []int32, values []float64, norm float64) float64 {
	return correlationAtOffsets(source.Pixels, (y*source.Width+x)*4, sourceOffsets(pixels, templateWidth, source.Width), values, norm)
}

func correlationAtOffsets(p []byte, base int, offsets []int32, values []float64, norm float64) float64 {
	var sumR, sumG, sumB, sumSquares, dot float64
	vi := 0
	for _, offset := range offsets {
		si := base + int(offset)
		r := float64(p[si])
		g := float64(p[si+1])
		b := float64(p[si+2])
		sumR += r
		sumG += g
		sumB += b
		sumSquares += r*r + g*g + b*b
		dot += r*values[vi] + g*values[vi+1] + b*values[vi+2]
		vi += 3
	}
	n := float64(len(offsets))
	variance := sumSquares - (sumR*sumR+sumG*sumG+sumB*sumB)/n
	if variance <= 0 || norm == 0 {
		return 0
	}
	return max(-1, min(1, dot/(norm*math.Sqrt(variance))))
}

// SearchTemplateScore finds a normalized-cross-correlation match of the
// template inside region. Unlike SearchTemplate, it tolerates uniform
// brightness and contrast changes (display tone mapping, HDR/SDR composition,
// dimming overlays) and returns the best verified candidate rather than the
// first tolerance hit. The default scan is approximate; Exhaustive guarantees
// the best location. A miss is (TemplateMatch{}, false, nil).
func SearchTemplateScore(source *Bitmap, region Rect, template *Template, options TemplateMatchOptions) (TemplateMatch, bool, error) {
	if source == nil || template == nil || !source.valid() || !template.bitmap.valid() {
		return TemplateMatch{}, false, ErrInvalidArgument
	}
	minScore := options.MinScore
	if minScore == 0 {
		minScore = defaultTemplateMinScore
	}
	if math.IsNaN(minScore) || minScore < 0 || minScore > 1 || options.Stride < 0 {
		return TemplateMatch{}, false, ErrInvalidArgument
	}
	t := template.bitmap
	c := template.correlationState()
	if c.norm == 0 || t.Width > source.Width || t.Height > source.Height {
		return TemplateMatch{}, false, nil
	}
	region, ok := source.clipRegion(region)
	if !ok {
		return TemplateMatch{}, false, nil
	}
	maxX, maxY := region.Right-t.Width, region.Bottom-t.Height
	if maxX < region.Left || maxY < region.Top {
		return TemplateMatch{}, false, nil
	}
	stride := options.Stride
	if stride == 0 {
		stride = 1
		if t.Width >= 40 && t.Height >= 40 {
			stride = 2
		}
	}
	stride = min(stride, max(source.Width, source.Height))
	if options.Exhaustive {
		stride = 1
	}
	sampleOffsets := sourceOffsets(c.samples, t.Width, source.Width)
	fullOffsets := sourceOffsets(c.pixels, t.Width, source.Width)
	rowBytes := source.Width * 4
	// Sample scores are noisier than full scores, so the coarse scan keeps a
	// margin below MinScore; every candidate is verified at full precision.
	// Once a strong match exists, weaker candidates are skipped.
	coarseMin := minScore - 0.15
	best := TemplateMatch{Score: -2}
	evaluated := make(map[int32]struct{})
	refine := func(cx, cy int) {
		for y := max(cy-stride+1, region.Top); y <= min(cy+stride-1, maxY); y++ {
			for x := max(cx-stride+1, region.Left); x <= min(cx+stride-1, maxX); x++ {
				key := int32(y)*int32(source.Width) + int32(x)
				if stride > 1 {
					if _, seen := evaluated[key]; seen {
						continue
					}
					evaluated[key] = struct{}{}
				}
				score := correlationAtOffsets(source.Pixels, y*rowBytes+x*4, fullOffsets, c.values, c.norm)
				if score > best.Score {
					best = TemplateMatch{Point: Point{x, y}, Score: score}
				}
			}
		}
	}
	for y := region.Top; y <= maxY; y += stride {
		base := y * rowBytes
		for x := region.Left; x <= maxX; x += stride {
			threshold := max(coarseMin, best.Score-0.1)
			if options.Exhaustive || correlationAtOffsets(source.Pixels, base+x*4, sampleOffsets, c.sample, c.sNorm) >= threshold {
				refine(x, y)
			}
		}
	}
	if best.Score < minScore {
		return TemplateMatch{}, false, nil
	}
	return best, true, nil
}

// SearchTemplateScoreWithCapture is the client-relative window form of
// SearchTemplateScore. The returned point is client-relative.
func (w Window) SearchTemplateScoreWithCapture(region Rect, template *Template, options TemplateMatchOptions, capture CaptureOptions) (TemplateMatch, bool, error) {
	origin := searchOrigin(region)
	capture.ClientOnly = true
	b, err := w.CaptureRegion(region, capture)
	if err != nil {
		return TemplateMatch{}, false, err
	}
	match, ok, err := SearchTemplateScore(b, Rect{}, template, options)
	if ok {
		match.Point.X += origin.X
		match.Point.Y += origin.Y
	}
	return match, ok, err
}
