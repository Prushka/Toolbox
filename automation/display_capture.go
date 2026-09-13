package automation

import "errors"

// captureWithDisplayCheck preserves ordinary capture behavior unless the
// caller opts in. A topology change during capture invalidates even a
// successful bitmap, including a black or stale GDI fallback frame.
func captureWithDisplayCheck(required bool, check func() error, capture func() (*Bitmap, error)) (*Bitmap, error) {
	if !required {
		return capture()
	}
	if err := check(); err != nil {
		return nil, err
	}
	bitmap, err := capture()
	if displayErr := check(); displayErr != nil {
		return nil, errors.Join(err, displayErr)
	}
	return bitmap, err
}
