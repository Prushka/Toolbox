//go:build windows

package automation

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetForegroundWindow           = user32.NewProc("GetForegroundWindow")
	procEnumWindows                   = user32.NewProc("EnumWindows")
	procIsWindowVisible               = user32.NewProc("IsWindowVisible")
	procIsIconic                      = user32.NewProc("IsIconic")
	procIsZoomed                      = user32.NewProc("IsZoomed")
	procShowWindow                    = user32.NewProc("ShowWindow")
	procSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop              = user32.NewProc("BringWindowToTop")
	procSetWindowPos                  = user32.NewProc("SetWindowPos")
	procGetWindowTextLength           = user32.NewProc("GetWindowTextLengthW")
	procGetWindowText                 = user32.NewProc("GetWindowTextW")
	procGetClassName                  = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId      = user32.NewProc("GetWindowThreadProcessId")
	procIsWindow                      = user32.NewProc("IsWindow")
	procPostMessage                   = user32.NewProc("PostMessageW")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procSetProcessDPIAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procOpenProcess                   = windows.NewLazySystemDLL("kernel32.dll").NewProc("OpenProcess")
	procCloseHandle                   = windows.NewLazySystemDLL("kernel32.dll").NewProc("CloseHandle")
	procQueryFullProcessImageName     = windows.NewLazySystemDLL("kernel32.dll").NewProc("QueryFullProcessImageNameW")
)

const (
	swHide                         = 0
	swMaximize                     = 3
	swMinimize                     = 6
	swRestore                      = 9
	swShow                         = 5
	swpNoZOrder                    = 0x0004
	swpShowWindow                  = 0x0040
	wmClose                        = 0x0010
	smCXScreen                     = 0
	smCYScreen                     = 1
	smXVirtualScreen               = 76
	smYVirtualScreen               = 77
	smCXVirtualScreen              = 78
	smCYVirtualScreen              = 79
	processQueryLimitedInformation = 0x1000
)

// Window is a lightweight handle wrapper. Handles are not owned and remain
// valid only while the native window exists.
type Window struct{ HWND HWND }

func (w Window) Handle() HWND { return w.HWND }
func (w Window) Valid() bool  { v, _, _ := procIsWindow.Call(uintptr(w.HWND)); return v != 0 }
func (w Window) Title() string {
	return getWindowString(procGetWindowTextLength, procGetWindowText, w.HWND)
}
func (w Window) Class() string { return getClass(w.HWND) }
func (w Window) PID() uint32 {
	var pid uint32
	procGetWindowThreadProcessId.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&pid)))
	return pid
}
func (w Window) ProcessPath() string {
	pid := w.PID()
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	buf := make([]uint16, 32768)
	n := uint32(len(buf))
	if ret, _, _ := procQueryFullProcessImageName.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); ret == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}
func (w Window) Rect() (Rect, error) {
	var r winRect
	if ret, _, _ := procGetWindowRect.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&r))); ret == 0 {
		return Rect{}, windows.GetLastError()
	}
	return Rect{int(r.Left), int(r.Top), int(r.Right), int(r.Bottom)}, nil
}
func (w Window) ClientRect() (Rect, error) {
	var r winRect
	if ret, _, _ := procGetClientRect.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&r))); ret == 0 {
		return Rect{}, windows.GetLastError()
	}
	return Rect{int(r.Left), int(r.Top), int(r.Right), int(r.Bottom)}, nil
}
func (w Window) ClientOrigin() (Point, error) {
	p := winPoint{}
	if ret, _, _ := procClientToScreen.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&p))); ret == 0 {
		return Point{}, windows.GetLastError()
	}
	return Point{int(p.X), int(p.Y)}, nil
}
func (w Window) IsVisible() bool   { v, _, _ := procIsWindowVisible.Call(uintptr(w.HWND)); return v != 0 }
func (w Window) IsMinimized() bool { v, _, _ := procIsIconic.Call(uintptr(w.HWND)); return v != 0 }
func (w Window) IsMaximized() bool { v, _, _ := procIsZoomed.Call(uintptr(w.HWND)); return v != 0 }
func (w Window) Activate() error {
	if w.IsMinimized() {
		procShowWindow.Call(uintptr(w.HWND), swRestore)
	}
	procBringWindowToTop.Call(uintptr(w.HWND))
	if ret, _, _ := procSetForegroundWindow.Call(uintptr(w.HWND)); ret == 0 {
		return fmt.Errorf("automation: Windows denied foreground activation")
	}
	return nil
}
func (w Window) Show() error     { return showWindow(w.HWND, swShow) }
func (w Window) Hide() error     { return showWindow(w.HWND, swHide) }
func (w Window) Minimize() error { return showWindow(w.HWND, swMinimize) }
func (w Window) Maximize() error { return showWindow(w.HWND, swMaximize) }
func (w Window) Restore() error  { return showWindow(w.HWND, swRestore) }
func (w Window) Close() error {
	if ret, _, _ := procPostMessage.Call(uintptr(w.HWND), wmClose, 0, 0); ret == 0 {
		return windows.GetLastError()
	}
	return nil
}
func (w Window) SetBounds(r Rect) error {
	if r.Empty() {
		return ErrInvalidRect
	}
	if ret, _, _ := procSetWindowPos.Call(uintptr(w.HWND), 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Width()), uintptr(r.Height()), swpNoZOrder|swpShowWindow); ret == 0 {
		return windows.GetLastError()
	}
	return nil
}
func (w Window) Move(p Point) error {
	r, e := w.Rect()
	if e != nil {
		return e
	}
	return w.SetBounds(Rect{p.X, p.Y, p.X + r.Width(), p.Y + r.Height()})
}
func (w Window) Resize(width, height int) error {
	r, e := w.Rect()
	if e != nil {
		return e
	}
	return w.SetBounds(Rect{r.Left, r.Top, r.Left + width, r.Top + height})
}
func (w Window) Toggle() error {
	if w.IsMinimized() {
		return w.Activate()
	}
	if active, _ := ActiveWindow(); active.HWND == w.HWND {
		return w.Minimize()
	}
	return w.Activate()
}
func showWindow(hwnd HWND, cmd uintptr) error {
	if valid, _, _ := procIsWindow.Call(uintptr(hwnd)); valid == 0 {
		return ErrNotFound
	}
	procShowWindow.Call(uintptr(hwnd), cmd)
	return nil
}

func (w Window) ClientToScreen(p Point) (Point, error) {
	n := winPoint{int32(p.X), int32(p.Y)}
	if ret, _, _ := procClientToScreen.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&n))); ret == 0 {
		return Point{}, windows.GetLastError()
	}
	return Point{int(n.X), int(n.Y)}, nil
}
func (w Window) ScreenToClient(p Point) (Point, error) {
	n := winPoint{int32(p.X), int32(p.Y)}
	if ret, _, _ := procScreenToClient.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&n))); ret == 0 {
		return Point{}, windows.GetLastError()
	}
	return Point{int(n.X), int(n.Y)}, nil
}

type WindowQuery struct {
	Title, TitleContains, Class, Process string
	PID                                  uint32
	VisibleOnly                          bool
}

func ActiveWindow() (Window, error) {
	h, _, _ := procGetForegroundWindow.Call()
	if h == 0 {
		return Window{}, ErrNotFound
	}
	return Window{HWND(h)}, nil
}
func FindWindows(q WindowQuery) ([]Window, error) {
	var out []Window
	cb := windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		w := Window{HWND(hwnd)}
		if q.VisibleOnly && !w.IsVisible() {
			return 1
		}
		if q.PID != 0 && w.PID() != q.PID {
			return 1
		}
		if q.Title != "" && !strings.EqualFold(w.Title(), q.Title) {
			return 1
		}
		if q.TitleContains != "" && !strings.Contains(strings.ToLower(w.Title()), strings.ToLower(q.TitleContains)) {
			return 1
		}
		if q.Class != "" && !strings.EqualFold(w.Class(), q.Class) {
			return 1
		}
		if q.Process != "" {
			p := w.ProcessPath()
			if !strings.EqualFold(p, q.Process) && !strings.EqualFold(pathBase(p), q.Process) {
				return 1
			}
		}
		out = append(out, w)
		return 1
	})
	ret, _, _ := procEnumWindows.Call(cb, 0)
	if ret == 0 {
		return nil, windows.GetLastError()
	}
	return out, nil
}
func FindWindow(q WindowQuery) (Window, error) {
	ws, e := FindWindows(q)
	if e != nil {
		return Window{}, e
	}
	if len(ws) == 0 {
		return Window{}, ErrNotFound
	}
	return ws[0], nil
}
func ToggleWindow(q WindowQuery) (Window, error) {
	w, e := FindWindow(q)
	if e != nil {
		return Window{}, e
	}
	return w, w.Toggle()
}
func pathBase(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '\\' || p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}
func getWindowString(lenProc, getProc *windows.LazyProc, hwnd HWND) string {
	n, _, _ := lenProc.Call(uintptr(hwnd))
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	r, _, _ := getProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:r])
}
func getClass(hwnd HWND) string {
	buf := make([]uint16, 256)
	r, _, _ := procGetClassName.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return windows.UTF16ToString(buf[:r])
}
func ScreenRect() Rect {
	return Rect{int(getMetric(smXVirtualScreen)), int(getMetric(smYVirtualScreen)), int(getMetric(smXVirtualScreen) + getMetric(smCXVirtualScreen)), int(getMetric(smYVirtualScreen) + getMetric(smCYVirtualScreen))}
}
func PrimaryScreenRect() Rect {
	return Rect{0, 0, int(getMetric(smCXScreen)), int(getMetric(smCYScreen))}
}
func getMetric(i int) int32 { v, _, _ := procGetSystemMetrics.Call(uintptr(i)); return int32(v) }
func SetDPIAware() error {
	if procSetProcessDPIAwarenessContext.Find() == nil {
		// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is the signed handle -4.
		if ret, _, callErr := procSetProcessDPIAwarenessContext.Call(^uintptr(3)); ret != 0 {
			return nil
		} else if callErr != windows.ERROR_INVALID_PARAMETER {
			return callErr
		}
	}
	if ret, _, _ := procSetProcessDPIAware.Call(); ret == 0 {
		return windows.GetLastError()
	}
	return nil
}
func WindowDPI(w Window) uint32 {
	if ret, _, _ := procGetDpiForWindow.Call(uintptr(w.HWND)); ret != 0 {
		return uint32(ret)
	}
	return 96
}
