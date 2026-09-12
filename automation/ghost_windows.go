//go:build windows

package automation

import "golang.org/x/sys/windows"

var (
	procGhostWindowFromHungWindow = windows.NewLazySystemDLL("user32.dll").NewProc("GhostWindowFromHungWindow")
	procHungWindowFromGhostWindow = windows.NewLazySystemDLL("user32.dll").NewProc("HungWindowFromGhostWindow")
)

// GhostWindow returns Windows' replacement for this hung window. These optional
// User32 exports are undocumented: unavailable exports return ErrUnsupported.
// A missing, stale, or nonreciprocal mapping returns ErrNotFound. Titles and
// geometry are never used to infer ownership. This method sends no messages.
func (w Window) GhostWindow() (Window, error) {
	if !w.Valid() || !w.IsHung() {
		return Window{}, ErrNotFound
	}
	if procGhostWindowFromHungWindow.Find() != nil || procHungWindowFromGhostWindow.Find() != nil {
		return Window{}, ErrUnsupported
	}
	pid := w.PID()
	h, _, _ := procGhostWindowFromHungWindow.Call(uintptr(w.Handle()))
	ghost := Window{HWND: HWND(h)}
	if h == 0 || ghost.Handle() == w.Handle() || !ghost.Valid() || ghost.Class() != "Ghost" {
		return Window{}, ErrNotFound
	}
	original, _, _ := procHungWindowFromGhostWindow.Call(h)
	if HWND(original) != w.Handle() || !w.Valid() || pid == 0 || w.PID() != pid || !w.IsHung() {
		return Window{}, ErrNotFound
	}
	return ghost, nil
}
