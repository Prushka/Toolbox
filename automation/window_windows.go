//go:build windows

package automation

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
func (w Window) Valid() bool {
	if w.HWND == 0 {
		return false
	}
	v, _, _ := procIsWindow.Call(uintptr(w.HWND))
	return v != 0
}
func (w Window) Title() string {
	return getWindowString(procGetWindowTextLength, procGetWindowText, w.HWND)
}
func (w Window) Class() string { return getClass(w.HWND) }
func (w Window) PID() uint32 {
	if w.HWND == 0 {
		return 0
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&pid)))
	return pid
}
func (w Window) ProcessPath() string {
	return processPath(w.PID())
}

func processPath(pid uint32) string {
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	buf := make([]uint16, 260)
	for {
		size := uint32(len(buf))
		n := size
		ret, _, callErr := procQueryFullProcessImageName.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n)))
		if ret != 0 {
			return windows.UTF16ToString(buf[:n])
		}
		if callErr != windows.ERROR_INSUFFICIENT_BUFFER {
			return ""
		}
		if len(buf) == 32768 {
			return ""
		}
		buf = make([]uint16, min(len(buf)*2, 32768))
	}
}
func (w Window) Rect() (Rect, error) {
	if !w.Valid() {
		return Rect{}, ErrNotFound
	}
	var r winRect
	if ret, _, callErr := procGetWindowRect.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&r))); ret == 0 {
		return Rect{}, winCallError(callErr, "GetWindowRect failed")
	}
	return Rect{int(r.Left), int(r.Top), int(r.Right), int(r.Bottom)}, nil
}
func (w Window) ClientRect() (Rect, error) {
	if !w.Valid() {
		return Rect{}, ErrNotFound
	}
	var r winRect
	if ret, _, callErr := procGetClientRect.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&r))); ret == 0 {
		return Rect{}, winCallError(callErr, "GetClientRect failed")
	}
	return Rect{int(r.Left), int(r.Top), int(r.Right), int(r.Bottom)}, nil
}
func (w Window) ClientOrigin() (Point, error) {
	if !w.Valid() {
		return Point{}, ErrNotFound
	}
	p := winPoint{}
	if ret, _, callErr := procClientToScreen.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&p))); ret == 0 {
		return Point{}, winCallError(callErr, "ClientToScreen failed")
	}
	return Point{int(p.X), int(p.Y)}, nil
}
func (w Window) IsVisible() bool   { v, _, _ := procIsWindowVisible.Call(uintptr(w.HWND)); return v != 0 }
func (w Window) IsMinimized() bool { v, _, _ := procIsIconic.Call(uintptr(w.HWND)); return v != 0 }
func (w Window) IsMaximized() bool { v, _, _ := procIsZoomed.Call(uintptr(w.HWND)); return v != 0 }
func (w Window) Activate() error {
	if !w.Valid() {
		return ErrNotFound
	}
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
	if !w.Valid() {
		return ErrNotFound
	}
	if ret, _, callErr := procPostMessage.Call(uintptr(w.HWND), wmClose, 0, 0); ret == 0 {
		return winCallError(callErr, "PostMessage(WM_CLOSE) failed")
	}
	return nil
}
func (w Window) SetBounds(r Rect) error {
	if !w.Valid() {
		return ErrNotFound
	}
	if r.Empty() {
		return ErrInvalidRect
	}
	left, ok := toWinInt32(r.Left)
	if !ok {
		return ErrInvalidArgument
	}
	top, ok := toWinInt32(r.Top)
	if !ok {
		return ErrInvalidArgument
	}
	width, height := r.Width(), r.Height()
	if width <= 0 || height <= 0 || width > 1<<31-1 || height > 1<<31-1 {
		return ErrInvalidRect
	}
	if ret, _, callErr := procSetWindowPos.Call(uintptr(w.HWND), 0, uintptr(left), uintptr(top), uintptr(width), uintptr(height), swpNoZOrder|swpShowWindow); ret == 0 {
		return winCallError(callErr, "SetWindowPos failed")
	}
	return nil
}
func (w Window) Move(p Point) error {
	r, e := w.Rect()
	if e != nil {
		return e
	}
	right, ok := checkedAddInt(p.X, r.Width())
	if !ok {
		return ErrInvalidArgument
	}
	bottom, ok := checkedAddInt(p.Y, r.Height())
	if !ok {
		return ErrInvalidArgument
	}
	return w.SetBounds(Rect{p.X, p.Y, right, bottom})
}
func (w Window) Resize(width, height int) error {
	if width <= 0 || height <= 0 {
		return ErrInvalidRect
	}
	r, e := w.Rect()
	if e != nil {
		return e
	}
	right, ok := checkedAddInt(r.Left, width)
	if !ok {
		return ErrInvalidArgument
	}
	bottom, ok := checkedAddInt(r.Top, height)
	if !ok {
		return ErrInvalidArgument
	}
	return w.SetBounds(Rect{r.Left, r.Top, right, bottom})
}
func (w Window) Toggle() error {
	if !w.Valid() {
		return ErrNotFound
	}
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
	if !w.Valid() {
		return Point{}, ErrNotFound
	}
	x, ok := toWinInt32(p.X)
	if !ok {
		return Point{}, ErrInvalidArgument
	}
	y, ok := toWinInt32(p.Y)
	if !ok {
		return Point{}, ErrInvalidArgument
	}
	n := winPoint{x, y}
	if ret, _, callErr := procClientToScreen.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&n))); ret == 0 {
		return Point{}, winCallError(callErr, "ClientToScreen failed")
	}
	return Point{int(n.X), int(n.Y)}, nil
}
func (w Window) ScreenToClient(p Point) (Point, error) {
	if !w.Valid() {
		return Point{}, ErrNotFound
	}
	x, ok := toWinInt32(p.X)
	if !ok {
		return Point{}, ErrInvalidArgument
	}
	y, ok := toWinInt32(p.Y)
	if !ok {
		return Point{}, ErrInvalidArgument
	}
	n := winPoint{x, y}
	if ret, _, callErr := procScreenToClient.Call(uintptr(w.HWND), uintptr(unsafe.Pointer(&n))); ret == 0 {
		return Point{}, winCallError(callErr, "ScreenToClient failed")
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
	return findWindows(q, false)
}

type windowEnumState struct {
	query         WindowQuery
	titleContains string
	processPaths  map[uint32]string
	windows       []Window
	firstOnly     bool
	stopped       bool
}

var (
	callbackStateID atomic.Uintptr
	callbackStates  sync.Map
)

func registerCallbackState(state any) (uintptr, func()) {
	for {
		id := callbackStateID.Add(1)
		if id == 0 {
			continue
		}
		if _, loaded := callbackStates.LoadOrStore(id, state); !loaded {
			return id, func() { callbackStates.Delete(id) }
		}
	}
}

var enumWindowsCallback = windows.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
	value, ok := callbackStates.Load(lparam)
	if !ok {
		return 0
	}
	state, ok := value.(*windowEnumState)
	if !ok {
		return 0
	}
	w := Window{HWND(hwnd)}
	q := state.query
	if q.VisibleOnly && !w.IsVisible() {
		return 1
	}
	pid := uint32(0)
	if q.PID != 0 {
		pid = w.PID()
		if pid != q.PID {
			return 1
		}
	}
	title := ""
	if q.Title != "" || state.titleContains != "" {
		title = w.Title()
	}
	if q.Title != "" && !strings.EqualFold(title, q.Title) {
		return 1
	}
	if state.titleContains != "" && !strings.Contains(strings.ToLower(title), state.titleContains) {
		return 1
	}
	if q.Class != "" && !strings.EqualFold(w.Class(), q.Class) {
		return 1
	}
	if q.Process != "" {
		if pid == 0 {
			pid = w.PID()
		}
		p, cached := state.processPaths[pid]
		if !cached {
			p = processPath(pid)
			state.processPaths[pid] = p
		}
		if !strings.EqualFold(p, q.Process) && !strings.EqualFold(pathBase(p), q.Process) {
			return 1
		}
	}
	state.windows = append(state.windows, w)
	if state.firstOnly {
		state.stopped = true
		return 0
	}
	return 1
})

func findWindows(q WindowQuery, firstOnly bool) ([]Window, error) {
	state := &windowEnumState{query: q, titleContains: strings.ToLower(q.TitleContains), firstOnly: firstOnly}
	if q.Process != "" {
		state.processPaths = make(map[uint32]string)
	}
	stateID, unregister := registerCallbackState(state)
	defer unregister()
	ret, _, callErr := procEnumWindows.Call(enumWindowsCallback, stateID)
	if ret == 0 && !state.stopped {
		return nil, winCallError(callErr, "EnumWindows failed")
	}
	return state.windows, nil
}
func FindWindow(q WindowQuery) (Window, error) {
	ws, e := findWindows(q, true)
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
	const maxWindowTextChars = 32768
	if n == 0 || n >= maxWindowTextChars {
		return ""
	}
	size := min(int(n)+16, maxWindowTextChars)
	for {
		buf := make([]uint16, size)
		r, _, _ := getProc.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if r == 0 {
			return ""
		}
		if r < uintptr(len(buf)-1) || size == maxWindowTextChars {
			return windows.UTF16ToString(buf[:r])
		}
		size = min(size*2, maxWindowTextChars)
	}
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
	left := int(getMetric(smXVirtualScreen))
	top := int(getMetric(smYVirtualScreen))
	width := int(getMetric(smCXVirtualScreen))
	height := int(getMetric(smCYVirtualScreen))
	right, ok := checkedAddInt(left, width)
	if !ok {
		return Rect{}
	}
	bottom, ok := checkedAddInt(top, height)
	if !ok {
		return Rect{}
	}
	return Rect{left, top, right, bottom}
}
func PrimaryScreenRect() Rect {
	return Rect{0, 0, int(getMetric(smCXScreen)), int(getMetric(smCYScreen))}
}
func getMetric(i int) int32 { v, _, _ := procGetSystemMetrics.Call(uintptr(i)); return int32(v) }

var (
	dpiOnce sync.Once
	dpiErr  error
)

func SetDPIAware() error {
	dpiOnce.Do(func() {
		if err := procSetProcessDPIAwarenessContext.Find(); err == nil {
			// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is the signed handle -4.
			if ret, _, callErr := procSetProcessDPIAwarenessContext.Call(^uintptr(3)); ret != 0 {
				return
			} else if callErr == windows.ERROR_ACCESS_DENIED {
				return
			} else if callErr != windows.ERROR_INVALID_PARAMETER {
				dpiErr = winCallError(callErr, "SetProcessDpiAwarenessContext failed")
				return
			}
		}
		if err := procSetProcessDPIAware.Find(); err != nil {
			dpiErr = err
			return
		}
		if ret, _, callErr := procSetProcessDPIAware.Call(); ret == 0 && callErr != windows.ERROR_ACCESS_DENIED {
			dpiErr = winCallError(callErr, "SetProcessDPIAware failed")
		}
	})
	return dpiErr
}
func WindowDPI(w Window) uint32 {
	if procGetDpiForWindow.Find() != nil {
		return 96
	}
	if ret, _, _ := procGetDpiForWindow.Call(uintptr(w.HWND)); ret != 0 {
		return uint32(ret)
	}
	return 96
}
