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

func (w Window) Capture(clientOnly bool) (*Bitmap, error) {
	return CaptureWindow(w.HWND, CaptureOptions{ClientOnly: clientOnly})
}
func (w Window) CaptureWith(o CaptureOptions) (*Bitmap, error) { return CaptureWindow(w.HWND, o) }
func (w Window) CaptureRegion(region Rect, o CaptureOptions) (*Bitmap, error) {
	return CaptureWindowRegion(w.HWND, region, o)
}
func (w Window) Pixel(x, y int) (RGB, error) { return PixelColorWindow(w.HWND, x, y, true) }
func (w Window) SearchPixel(region Rect, want RGB, t ColorTolerance) (Point, bool, error) {
	origin := searchOrigin(region)
	b, e := w.CaptureRegion(region, CaptureOptions{ClientOnly: true})
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
	t, e := CompileTemplate(tpl, o)
	if e != nil {
		return Point{}, false, e
	}
	return w.SearchTemplate(region, t)
}
func (w Window) SearchTemplate(region Rect, t *Template) (Point, bool, error) {
	origin := searchOrigin(region)
	b, e := w.CaptureRegion(region, CaptureOptions{ClientOnly: true})
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
	t, e := LoadTemplate(path, o)
	if e != nil {
		return Point{}, false, e
	}
	return w.SearchTemplate(region, t)
}
func (w Window) SearchImageFileSpec(region Rect, spec string) (Point, bool, error) {
	o, path, e := ParseImageSearchOptions(spec)
	if e != nil {
		return Point{}, false, e
	}
	if path == "" {
		return Point{}, false, ErrInvalidArgument
	}
	return w.SearchImageFile(region, path, o)
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
		if ok {
			return nil
		}
	}
}
