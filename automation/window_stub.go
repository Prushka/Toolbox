//go:build !windows

package automation

import (
	"context"
	"time"
)

type Window struct{ HWND HWND }
type WindowQuery struct {
	Title, TitleContains, Class, Process string
	PID                                  uint32
	VisibleOnly                          bool
}

func ActiveWindow() (Window, error)                  { return Window{}, ErrUnsupported }
func FindWindows(WindowQuery) ([]Window, error)      { return nil, ErrUnsupported }
func FindWindow(WindowQuery) (Window, error)         { return Window{}, ErrUnsupported }
func ToggleWindow(WindowQuery) (Window, error)       { return Window{}, ErrUnsupported }
func ScreenRect() Rect                               { return Rect{} }
func PrimaryScreenRect() Rect                        { return Rect{} }
func SetDPIAware() error                             { return ErrUnsupported }
func WindowDPI(Window) uint32                        { return 96 }
func (w Window) Handle() HWND                        { return w.HWND }
func (w Window) ClientToScreen(Point) (Point, error) { return Point{}, ErrUnsupported }
func (w Window) ScreenToClient(Point) (Point, error) { return Point{}, ErrUnsupported }
func (w Window) Valid() bool                         { return false }
func (w Window) Title() string                       { return "" }
func (w Window) Class() string                       { return "" }
func (w Window) PID() uint32                         { return 0 }
func (w Window) ProcessPath() string                 { return "" }
func (w Window) Rect() (Rect, error)                 { return Rect{}, ErrUnsupported }
func (w Window) ClientRect() (Rect, error)           { return Rect{}, ErrUnsupported }
func (w Window) ClientOrigin() (Point, error)        { return Point{}, ErrUnsupported }
func (w Window) IsVisible() bool                     { return false }
func (w Window) IsMinimized() bool                   { return false }
func (w Window) IsMaximized() bool                   { return false }
func (w Window) Activate() error                     { return ErrUnsupported }
func (w Window) EnsureActive(context.Context, time.Duration) error {
	return ErrUnsupported
}
func (w Window) Show() error           { return ErrUnsupported }
func (w Window) Hide() error           { return ErrUnsupported }
func (w Window) Minimize() error       { return ErrUnsupported }
func (w Window) Maximize() error       { return ErrUnsupported }
func (w Window) Restore() error        { return ErrUnsupported }
func (w Window) Close() error          { return ErrUnsupported }
func (w Window) SetBounds(Rect) error  { return ErrUnsupported }
func (w Window) Move(Point) error      { return ErrUnsupported }
func (w Window) Resize(int, int) error { return ErrUnsupported }
func (w Window) Toggle() error         { return ErrUnsupported }
