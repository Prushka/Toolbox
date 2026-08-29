package automation

import (
	"context"
	"image"
	"time"
)

// SearchPixel follows AHK PixelSearch scan order. Both endpoints are included;
// reversing either endpoint reverses that axis' scan direction.
func SearchPixel(b *Bitmap, start, end Point, want RGB, tolerance ColorTolerance) (Point, bool) {
	if !b.valid() {
		return Point{}, false
	}
	dx, dy := 1, 1
	if end.X < start.X {
		dx = -1
	}
	if end.Y < start.Y {
		dy = -1
	}
	if dx > 0 {
		if start.X < 0 {
			start.X = 0
		}
		if end.X >= b.Width {
			end.X = b.Width - 1
		}
		if start.X > end.X {
			return Point{}, false
		}
	} else {
		if start.X >= b.Width {
			start.X = b.Width - 1
		}
		if end.X < 0 {
			end.X = 0
		}
		if start.X < end.X {
			return Point{}, false
		}
	}
	if dy > 0 {
		if start.Y < 0 {
			start.Y = 0
		}
		if end.Y >= b.Height {
			end.Y = b.Height - 1
		}
		if start.Y > end.Y {
			return Point{}, false
		}
	} else {
		if start.Y >= b.Height {
			start.Y = b.Height - 1
		}
		if end.Y < 0 {
			end.Y = 0
		}
		if start.Y < end.Y {
			return Point{}, false
		}
	}
	for y := start.Y; ; y += dy {
		i := (y*b.Width + start.X) * 4
		for x := start.X; ; x += dx {
			if abs(b.Pixels[i], want.R) <= tolerance.R &&
				abs(b.Pixels[i+1], want.G) <= tolerance.G &&
				abs(b.Pixels[i+2], want.B) <= tolerance.B {
				return Point{x, y}, true
			}
			if x == end.X {
				break
			}
			i += dx * 4
		}
		if y == end.Y {
			break
		}
	}
	return Point{}, false
}

// SearchPixelRect scans a half-open region from top-left to bottom-right.
func SearchPixelRect(b *Bitmap, region Rect, want RGB, tolerance ColorTolerance) (Point, bool) {
	if !b.valid() {
		return Point{}, false
	}
	if region == (Rect{}) {
		region = Rect{0, 0, b.Width, b.Height}
	}
	region = region.Normalize()
	if region.Empty() || region.Right <= 0 || region.Bottom <= 0 || region.Left >= b.Width || region.Top >= b.Height {
		return Point{}, false
	}
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
	return SearchPixel(b, Point{region.Left, region.Top}, Point{region.Right - 1, region.Bottom - 1}, want, tolerance)
}

func PixelMatches(got, want RGB, variation uint8) bool {
	return got.Matches(want, Tolerance(variation))
}
func PixelMatchesTolerance(got, want RGB, t ColorTolerance) bool { return got.Matches(want, t) }

// RelativePoint scales a point from a reference client size to a current one.
func RelativePoint(p Point, reference, current image.Point) Point {
	if reference.X <= 0 || reference.Y <= 0 || current.X < 0 || current.Y < 0 {
		return Point{}
	}
	x, xOK := checkedMulInt(p.X, current.X)
	y, yOK := checkedMulInt(p.Y, current.Y)
	if !xOK || !yOK {
		return Point{}
	}
	return Point{x / reference.X, y / reference.Y}
}

// AspectFitRect returns the largest centered rectangle in current with the
// same aspect ratio as reference. It is useful for applications that preserve
// their logical canvas and add letterboxing or pillarboxing at other sizes.
func AspectFitRect(reference, current image.Point) Rect {
	if reference.X <= 0 || reference.Y <= 0 || current.X <= 0 || current.Y <= 0 {
		return Rect{}
	}
	widthLimitedLeft, leftOK := checkedMulInt(current.X, reference.Y)
	widthLimitedRight, rightOK := checkedMulInt(current.Y, reference.X)
	if !leftOK || !rightOK {
		return Rect{}
	}
	if widthLimitedLeft <= widthLimitedRight {
		height, ok := roundedScale(reference.Y, current.X, reference.X)
		if !ok {
			return Rect{}
		}
		top := (current.Y - height) / 2
		return Rect{Left: 0, Top: top, Right: current.X, Bottom: top + height}
	}
	width, ok := roundedScale(reference.X, current.Y, reference.Y)
	if !ok {
		return Rect{}
	}
	left := (current.X - width) / 2
	return Rect{Left: left, Top: 0, Right: left + width, Bottom: current.Y}
}

// AspectFitPoint maps a point from reference coordinates into a centered,
// uniformly scaled canvas in current. Unlike RelativePoint, it never distorts
// one axis independently from the other.
func AspectFitPoint(p Point, reference, current image.Point) Point {
	fit := AspectFitRect(reference, current)
	if fit.Empty() {
		return Point{}
	}
	widthLimitedLeft, leftOK := checkedMulInt(current.X, reference.Y)
	widthLimitedRight, rightOK := checkedMulInt(current.Y, reference.X)
	if !leftOK || !rightOK {
		return Point{}
	}
	if widthLimitedLeft <= widthLimitedRight {
		x, xOK := roundedScale(p.X, current.X, reference.X)
		y, yOK := roundedScale(p.Y, current.X, reference.X)
		if !xOK || !yOK {
			return Point{}
		}
		return Point{X: x, Y: fit.Top + y}
	}
	x, xOK := roundedScale(p.X, current.Y, reference.Y)
	y, yOK := roundedScale(p.Y, current.Y, reference.Y)
	if !xOK || !yOK {
		return Point{}
	}
	return Point{X: fit.Left + x, Y: y}
}

func roundedScale(value, numerator, denominator int) (int, bool) {
	scaled, ok := checkedMulInt(value, numerator)
	if !ok {
		return 0, false
	}
	half := denominator / 2
	scaled, ok = checkedAddInt(scaled, half)
	if !ok {
		return 0, false
	}
	return scaled / denominator, true
}

func (w Window) Capture(clientOnly bool) (*Bitmap, error) {
	return CaptureWindow(w.HWND, CaptureOptions{ClientOnly: clientOnly})
}
func (w Window) CaptureWith(o CaptureOptions) (*Bitmap, error) { return CaptureWindow(w.HWND, o) }
func (w Window) CaptureRegion(region Rect, o CaptureOptions) (*Bitmap, error) {
	return CaptureWindowRegion(w.HWND, region, o)
}
func (w Window) Pixel(x, y int) (RGB, error) { return PixelColorWindow(w.HWND, x, y, true) }
func (w Window) SearchPixel(region Rect, want RGB, t ColorTolerance) (Point, bool, error) {
	return w.SearchPixelWithCapture(region, want, t, CaptureOptions{})
}
func (w Window) SearchPixelWithCapture(region Rect, want RGB, t ColorTolerance, capture CaptureOptions) (Point, bool, error) {
	origin := searchOrigin(region)
	capture.ClientOnly = true
	b, e := w.CaptureRegion(region, capture)
	if e != nil {
		return Point{}, false, e
	}
	p, ok := SearchPixelRect(b, Rect{}, want, t)
	if ok {
		p.X += origin.X
		p.Y += origin.Y
	}
	return p, ok, nil
}
func (w Window) SearchImage(region Rect, tpl image.Image, o ImageSearchOptions) (Point, bool, error) {
	return w.SearchImageWithCapture(region, tpl, o, CaptureOptions{})
}
func (w Window) SearchImageWithCapture(region Rect, tpl image.Image, o ImageSearchOptions, capture CaptureOptions) (Point, bool, error) {
	t, e := CompileTemplate(tpl, o)
	if e != nil {
		return Point{}, false, e
	}
	return w.SearchTemplateWithCapture(region, t, capture)
}
func (w Window) SearchTemplate(region Rect, t *Template) (Point, bool, error) {
	return w.SearchTemplateWithCapture(region, t, CaptureOptions{})
}
func (w Window) SearchTemplateWithCapture(region Rect, t *Template, capture CaptureOptions) (Point, bool, error) {
	origin := searchOrigin(region)
	capture.ClientOnly = true
	b, e := w.CaptureRegion(region, capture)
	if e != nil {
		return Point{}, false, e
	}
	p, ok, e := SearchTemplate(b, Rect{}, t)
	if ok {
		p.X += origin.X
		p.Y += origin.Y
	}
	return p, ok, e
}
func (w Window) SearchImageFile(region Rect, path string, o ImageSearchOptions) (Point, bool, error) {
	return w.SearchImageFileWithCapture(region, path, o, CaptureOptions{})
}
func (w Window) SearchImageFileWithCapture(region Rect, path string, o ImageSearchOptions, capture CaptureOptions) (Point, bool, error) {
	t, e := LoadTemplate(path, o)
	if e != nil {
		return Point{}, false, e
	}
	return w.SearchTemplateWithCapture(region, t, capture)
}
func (w Window) SearchImageFileSpec(region Rect, spec string) (Point, bool, error) {
	return w.SearchImageFileSpecWithCapture(region, spec, CaptureOptions{})
}
func (w Window) SearchImageFileSpecWithCapture(region Rect, spec string, capture CaptureOptions) (Point, bool, error) {
	o, path, e := ParseImageSearchOptions(spec)
	if e != nil {
		return Point{}, false, e
	}
	if path == "" {
		return Point{}, false, ErrInvalidArgument
	}
	return w.SearchImageFileWithCapture(region, path, o, capture)
}

func searchOrigin(r Rect) Point {
	if r == (Rect{}) {
		return Point{}
	}
	r = r.Normalize()
	if r.Empty() {
		return Point{}
	}
	if r.Left < 0 {
		r.Left = 0
	}
	if r.Top < 0 {
		r.Top = 0
	}
	return Point{r.Left, r.Top}
}

func (w Window) ClientSize() (image.Point, error) {
	r, e := w.ClientRect()
	if e != nil {
		return image.Point{}, e
	}
	return image.Point{r.Width(), r.Height()}, nil
}
func (w Window) ScalePoint(p Point, reference image.Point) (Point, error) {
	size, e := w.ClientSize()
	if e != nil {
		return Point{}, e
	}
	return RelativePoint(p, reference, size), nil
}

// FrameStabilityOptions controls how long a captured region must stop changing
// before it is considered ready for input. ConsecutiveFrames includes the
// initial frame. MaxChangedFraction permits small animated details while the
// rest of the region is stable.
type FrameStabilityOptions struct {
	Interval           time.Duration
	ConsecutiveFrames  int
	PixelTolerance     uint8
	MaxChangedFraction float64
}

// BitmapsSimilar reports whether two same-sized bitmaps differ in no more than
// the allowed fraction of pixels. Alpha is ignored because Windows capture
// backends commonly normalize it.
func BitmapsSimilar(a, b *Bitmap, pixelTolerance uint8, maxChangedFraction float64) bool {
	if a == nil || b == nil || !a.valid() || !b.valid() ||
		a.Width != b.Width || a.Height != b.Height ||
		maxChangedFraction < 0 || maxChangedFraction > 1 {
		return false
	}
	pixels := a.Width * a.Height
	maxChanged := int(float64(pixels) * maxChangedFraction)
	changed := 0
	for offset := 0; offset < pixels*4; offset += 4 {
		if abs(a.Pixels[offset], b.Pixels[offset]) > pixelTolerance ||
			abs(a.Pixels[offset+1], b.Pixels[offset+1]) > pixelTolerance ||
			abs(a.Pixels[offset+2], b.Pixels[offset+2]) > pixelTolerance {
			changed++
			if changed > maxChanged {
				return false
			}
		}
	}
	return true
}

// WaitForStableBitmap polls capture until the requested number of consecutive
// similar frames has been observed. It is useful for animation-driven UIs that
// expose no separate readiness element.
func WaitForStableBitmap(ctx context.Context, capture func() (*Bitmap, error), options FrameStabilityOptions) error {
	if !validFrameWait(ctx, capture, options) {
		return ErrInvalidArgument
	}
	options = defaultFrameStabilityOptions(options)
	if err := ctx.Err(); err != nil {
		return err
	}
	previous, err := capture()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if previous == nil || !previous.valid() {
		return ErrInvalidArgument
	}
	stableFrames := 1
	if stableFrames >= options.ConsecutiveFrames {
		return nil
	}
	for {
		if err := Sleep(ctx, options.Interval); err != nil {
			return err
		}
		current, err := capture()
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == nil || !current.valid() {
			return ErrInvalidArgument
		}
		if BitmapsSimilar(previous, current, options.PixelTolerance, options.MaxChangedFraction) {
			stableFrames++
		} else {
			stableFrames = 1
		}
		previous = current
		if stableFrames >= options.ConsecutiveFrames {
			return nil
		}
	}
}

// WaitForBitmapChange polls capture until the current frame differs from the
// initial frame. It is useful as a render-progress gate before input: a caller
// can distinguish an application that is presenting frames from one whose
// visible content is temporarily frozen.
func WaitForBitmapChange(ctx context.Context, capture func() (*Bitmap, error), options FrameStabilityOptions) error {
	if !validFrameWait(ctx, capture, options) {
		return ErrInvalidArgument
	}
	options = defaultFrameStabilityOptions(options)
	if err := ctx.Err(); err != nil {
		return err
	}
	initial, err := capture()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if initial == nil || !initial.valid() {
		return ErrInvalidArgument
	}
	for {
		if err := Sleep(ctx, options.Interval); err != nil {
			return err
		}
		current, err := capture()
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == nil || !current.valid() ||
			current.Width != initial.Width || current.Height != initial.Height {
			return ErrInvalidArgument
		}
		if !BitmapsSimilar(initial, current, options.PixelTolerance, options.MaxChangedFraction) {
			return nil
		}
	}
}

// WaitForBitmapTransition captures an initial frame, invokes action, waits for
// the capture to differ from that frame, then waits for the changed state to
// stabilize. This prevents a caller from mistaking a stale, already-stable UI
// for completion of the action it just dispatched.
func WaitForBitmapTransition(
	ctx context.Context,
	capture func() (*Bitmap, error),
	action func() error,
	options FrameStabilityOptions,
) error {
	if !validFrameWait(ctx, capture, options) || action == nil {
		return ErrInvalidArgument
	}
	options = defaultFrameStabilityOptions(options)
	if err := ctx.Err(); err != nil {
		return err
	}
	initial, err := capture()
	if err != nil {
		return err
	}
	// Native capture can block while Windows copies a frozen game frame. The
	// context may expire during that call; never dispatch a non-idempotent
	// action from an observation that was completed after its deadline.
	if err := ctx.Err(); err != nil {
		return err
	}
	if initial == nil || !initial.valid() {
		return ErrInvalidArgument
	}
	if err := action(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	changed := false
	stableFrames := 0
	var previous *Bitmap
	for {
		if err := Sleep(ctx, options.Interval); err != nil {
			return err
		}
		current, err := capture()
		if err != nil {
			return err
		}
		// A slow capture may return after cancellation. Do not interpret its
		// pixels as a successful transition, and do not continue polling with a
		// stale deadline.
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == nil || !current.valid() ||
			current.Width != initial.Width || current.Height != initial.Height {
			return ErrInvalidArgument
		}
		if !changed {
			if BitmapsSimilar(initial, current, options.PixelTolerance, options.MaxChangedFraction) {
				continue
			}
			changed = true
			stableFrames = 1
			previous = current
			if stableFrames >= options.ConsecutiveFrames {
				return nil
			}
			continue
		}
		if BitmapsSimilar(previous, current, options.PixelTolerance, options.MaxChangedFraction) {
			stableFrames++
		} else {
			stableFrames = 1
		}
		previous = current
		if stableFrames >= options.ConsecutiveFrames {
			return nil
		}
	}
}

func validFrameWait(ctx context.Context, capture func() (*Bitmap, error), options FrameStabilityOptions) bool {
	return ctx != nil && capture != nil && options.Interval >= 0 && options.ConsecutiveFrames >= 0 &&
		options.MaxChangedFraction >= 0 && options.MaxChangedFraction <= 1
}

func defaultFrameStabilityOptions(options FrameStabilityOptions) FrameStabilityOptions {
	if options.Interval == 0 {
		options.Interval = 100 * time.Millisecond
	}
	if options.ConsecutiveFrames == 0 {
		options.ConsecutiveFrames = 2
	}
	return options
}

// WaitForStableRegion captures a client-relative window region until its
// frames satisfy options.
func (w Window) WaitForStableRegion(ctx context.Context, region Rect, options FrameStabilityOptions, captureOptions CaptureOptions) error {
	captureOptions.ClientOnly = true
	return WaitForStableBitmap(ctx, func() (*Bitmap, error) {
		return w.CaptureRegion(region, captureOptions)
	}, options)
}

// WaitForRegionTransition is the client-relative window form of
// WaitForBitmapTransition.
func (w Window) WaitForRegionTransition(
	ctx context.Context,
	region Rect,
	action func() error,
	options FrameStabilityOptions,
	captureOptions CaptureOptions,
) error {
	captureOptions.ClientOnly = true
	return WaitForBitmapTransition(ctx, func() (*Bitmap, error) {
		return w.CaptureRegion(region, captureOptions)
	}, action, options)
}

// WaitUntil polls without busy-waiting and stops promptly on cancellation.
func WaitUntil(ctx context.Context, interval time.Duration, predicate func() (bool, error)) error {
	if ctx == nil {
		return ErrInvalidArgument
	}
	if predicate == nil {
		return ErrInvalidArgument
	}
	if interval <= 0 {
		interval = time.Millisecond
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ok, err := predicate()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if ok {
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		ok, err = predicate()
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
}
