//go:build !windows

package automation

type CaptureMethod uint8

const (
	CaptureVisible CaptureMethod = iota
	CapturePrintWindow
	CaptureAuto
)

type CaptureOptions struct {
	RequireActiveDisplay bool
	ClientOnly           bool
	Method               CaptureMethod
}

func CaptureScreen(Rect) (*Bitmap, error)                             { return nil, ErrUnsupported }
func CaptureWindow(HWND, CaptureOptions) (*Bitmap, error)             { return nil, ErrUnsupported }
func CaptureWindowRegion(HWND, Rect, CaptureOptions) (*Bitmap, error) { return nil, ErrUnsupported }
func PixelColor(int, int) (RGB, error)                                { return RGB{}, ErrUnsupported }
func PixelColorWindow(HWND, int, int, bool) (RGB, error)              { return RGB{}, ErrUnsupported }
func CursorPosition() (Point, error)                                  { return Point{}, ErrUnsupported }
