//go:build !windows

package automation

type DisplayMode struct{ Width, Height, BitsPerPixel, Frequency int }
type Monitor struct {
	Rect, WorkArea Rect
	Primary        bool
}

func CurrentDisplayMode() (DisplayMode, error) { return DisplayMode{}, ErrUnsupported }
func DisplayModes() ([]DisplayMode, error)     { return nil, ErrUnsupported }
func SetDisplayMode(DisplayMode, bool) error   { return ErrUnsupported }
func RestoreDisplayMode() error                { return ErrUnsupported }
func Monitors() ([]Monitor, error)             { return nil, ErrUnsupported }
