//go:build windows

package automation

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	gdi32                  = windows.NewLazySystemDLL("gdi32.dll")
	procGetDC              = user32.NewProc("GetDC")
	procReleaseDC          = user32.NewProc("ReleaseDC")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procGetWindowRect      = user32.NewProc("GetWindowRect")
	procClientToScreen     = user32.NewProc("ClientToScreen")
	procScreenToClient     = user32.NewProc("ScreenToClient")
	procPrintWindow        = user32.NewProc("PrintWindow")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procGdiFlush           = gdi32.NewProc("GdiFlush")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procGetPixel           = gdi32.NewProc("GetPixel")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
)

const (
	srcCopy             = 0x00CC0020
	dibRGBColors        = 0
	pwClientOnly        = 0x00000001
	pwRenderFullContent = 0x00000002
)

type winRect struct{ Left, Top, Right, Bottom int32 }
type bitmapInfoHeader struct {
	Size                         uint32
	Width, Height                int32
	Planes, BitCount             uint16
	Compression, SizeImage       uint32
	XPelsPerMeter, YPelsPerMeter int32
	ClrUsed, ClrImportant        uint32
}
type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

func CaptureScreen(rect Rect) (*Bitmap, error) {
	if rect.Left > rect.Right || rect.Top > rect.Bottom {
		rect = rect.Normalize()
	}
	if rect.Empty() {
		return nil, ErrInvalidRect
	}
	left, ok := toWinInt32(rect.Left)
	if !ok {
		return nil, ErrInvalidArgument
	}
	top, ok := toWinInt32(rect.Top)
	if !ok {
		return nil, ErrInvalidArgument
	}
	right, ok := toWinInt32(rect.Right)
	if !ok {
		return nil, ErrInvalidArgument
	}
	bottom, ok := toWinInt32(rect.Bottom)
	if !ok {
		return nil, ErrInvalidArgument
	}
	w, h, err := captureDimensions(left, right, top, bottom)
	if err != nil {
		return nil, err
	}
	// GetDC and ReleaseDC must execute on the same OS thread. The DIB copy
	// also flushes that thread's GDI batch before reading its pixel storage.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hdc, _, callErr := procGetDC.Call(0)
	if hdc == 0 {
		return nil, winCallError(callErr, "GetDC failed")
	}
	defer procReleaseDC.Call(0, hdc)
	return captureFromDC(hdc, int(left), int(top), w, h)
}

// CaptureWindow captures a client or full window. The default visible method
// translates bounds to screen coordinates and does not message or open the
// target process. PrintWindow is synchronous and may block in the target's
// window procedure.
func CaptureWindow(hwnd HWND, opts CaptureOptions) (*Bitmap, error) {
	return captureWithDisplayCheck(opts.RequireActiveDisplay, RequireActiveDisplay, func() (*Bitmap, error) {
		return captureWindow(hwnd, opts)
	})
}

func captureWindow(hwnd HWND, opts CaptureOptions) (*Bitmap, error) {
	if hwnd == 0 {
		return nil, ErrNotFound
	}
	if !validCaptureMethod(opts.Method) {
		return nil, ErrInvalidArgument
	}
	var r winRect
	var ok uintptr
	var callErr error
	operation := "GetWindowRect failed"
	if opts.ClientOnly {
		operation = "GetClientRect failed"
		ok, _, callErr = procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	} else {
		ok, _, callErr = procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	}
	if ok == 0 {
		return nil, winCallError(callErr, operation)
	}
	w, h, err := captureDimensions(r.Left, r.Right, r.Top, r.Bottom)
	if err != nil {
		return nil, err
	}
	if opts.Method != CaptureVisible {
		b, printErr := capturePrintWindow(hwnd, w, h, opts.ClientOnly)
		if printErr == nil || opts.Method == CapturePrintWindow {
			return b, printErr
		}
	}
	var origin winPoint
	if opts.ClientOnly {
		if ret, _, callErr := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&origin))); ret == 0 {
			return nil, winCallError(callErr, "ClientToScreen failed")
		}
	} else {
		origin.X, origin.Y = r.Left, r.Top
	}
	right, valid := checkedAddInt(int(origin.X), w)
	if !valid {
		return nil, ErrInvalidArgument
	}
	bottom, valid := checkedAddInt(int(origin.Y), h)
	if !valid {
		return nil, ErrInvalidArgument
	}
	visible := Rect{int(origin.X), int(origin.Y), right, bottom}
	return CaptureScreen(visible)
}

// CaptureWindowRegion captures a client-relative region without capturing the
// rest of the window. Regions are clipped to the client area.
func CaptureWindowRegion(hwnd HWND, region Rect, opts CaptureOptions) (*Bitmap, error) {
	return captureWithDisplayCheck(opts.RequireActiveDisplay, RequireActiveDisplay, func() (*Bitmap, error) {
		return captureWindowRegion(hwnd, region, opts)
	})
}

func captureWindowRegion(hwnd HWND, region Rect, opts CaptureOptions) (*Bitmap, error) {
	if hwnd == 0 {
		return nil, ErrNotFound
	}
	if !validCaptureMethod(opts.Method) {
		return nil, ErrInvalidArgument
	}
	var cr winRect
	if ret, _, callErr := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&cr))); ret == 0 {
		return nil, winCallError(callErr, "GetClientRect failed")
	}
	w, h, err := captureDimensions(cr.Left, cr.Right, cr.Top, cr.Bottom)
	if err != nil {
		return nil, err
	}
	bounds := Rect{0, 0, w, h}
	if region == (Rect{}) {
		region = bounds
	}
	region = region.Normalize()
	if region.Left < 0 {
		region.Left = 0
	}
	if region.Top < 0 {
		region.Top = 0
	}
	if region.Right > bounds.Right {
		region.Right = bounds.Right
	}
	if region.Bottom > bounds.Bottom {
		region.Bottom = bounds.Bottom
	}
	if region.Empty() {
		return nil, ErrInvalidRect
	}
	if opts.Method != CaptureVisible {
		b, printErr := capturePrintWindowRegion(hwnd, w, h, true, region)
		if printErr == nil || opts.Method == CapturePrintWindow {
			return b, printErr
		}
	}
	var p winPoint
	if ret, _, callErr := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p))); ret == 0 {
		return nil, winCallError(callErr, "ClientToScreen failed")
	}
	left, ok := checkedAddInt(int(p.X), region.Left)
	if !ok {
		return nil, ErrInvalidArgument
	}
	top, ok := checkedAddInt(int(p.Y), region.Top)
	if !ok {
		return nil, ErrInvalidArgument
	}
	right, ok := checkedAddInt(int(p.X), region.Right)
	if !ok {
		return nil, ErrInvalidArgument
	}
	bottom, ok := checkedAddInt(int(p.Y), region.Bottom)
	if !ok {
		return nil, ErrInvalidArgument
	}
	return CaptureScreen(Rect{left, top, right, bottom})
}

type CaptureMethod uint8

const (
	CaptureVisible CaptureMethod = iota
	CapturePrintWindow
	CaptureAuto
)

type CaptureOptions struct {
	// RequireActiveDisplay checks Windows display availability before and after
	// capture. The zero value preserves capture without a topology requirement.
	RequireActiveDisplay bool
	ClientOnly           bool
	Method               CaptureMethod
}
type winPoint struct{ X, Y int32 }

func validCaptureMethod(m CaptureMethod) bool { return m <= CaptureAuto }

func toWinInt32(v int) (int32, bool) {
	if int64(v) < int64(-1<<31) || int64(v) > int64(1<<31-1) {
		return 0, false
	}
	return int32(v), true
}

func captureDimensions(left, right, top, bottom int32) (int, int, error) {
	w := int64(right) - int64(left)
	h := int64(bottom) - int64(top)
	if w <= 0 || h <= 0 || w > int64(maxInt) || h > int64(maxInt) || w > int64(1<<31-1) || h > int64(1<<31-1) {
		return 0, 0, ErrInvalidRect
	}
	bytes, ok := bitmapByteLen(int(w), int(h))
	if !ok || bytes > maxBitmapBytes {
		return 0, 0, ErrInvalidRect
	}
	return int(w), int(h), nil
}

func winCallError(callErr error, operation string) error {
	if callErr != nil && callErr != windows.ERROR_SUCCESS {
		return fmt.Errorf("automation: %s: %w", operation, callErr)
	}
	return fmt.Errorf("automation: %s", operation)
}

func capturePrintWindow(hwnd HWND, w, h int, client bool) (*Bitmap, error) {
	return capturePrintWindowRegion(hwnd, w, h, client, Rect{0, 0, w, h})
}

func capturePrintWindowRegion(hwnd HWND, w, h int, client bool, region Rect) (*Bitmap, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dst, bits, cleanup, err := makeDIB(w, h)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := procPrintWindow.Find(); err != nil {
		return nil, err
	}
	flags := uintptr(pwRenderFullContent)
	if client {
		flags |= pwClientOnly
	}
	if ret, _, callErr := procPrintWindow.Call(uintptr(hwnd), dst, flags); ret == 0 {
		return nil, winCallError(callErr, "PrintWindow failed")
	}
	if err := flushCaptureGDI(); err != nil {
		return nil, err
	}
	return dibRegionToBitmap(bits, w, h, region)
}

func captureFromDC(src uintptr, left, top, w, h int) (*Bitmap, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	left32, ok := toWinInt32(left)
	if !ok {
		return nil, ErrInvalidArgument
	}
	top32, ok := toWinInt32(top)
	if !ok {
		return nil, ErrInvalidArgument
	}
	dst, bits, cleanup, err := makeDIB(w, h)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if ret, _, callErr := procBitBlt.Call(dst, 0, 0, uintptr(w), uintptr(h), src, uintptr(left32), uintptr(top32), srcCopy); ret == 0 {
		return nil, winCallError(callErr, "BitBlt failed")
	}
	if err := flushCaptureGDI(); err != nil {
		return nil, err
	}
	return dibToBitmap(bits, w, h)
}

// CreateDIBSection requires GDI writes to finish before direct access to bits.
// GdiFlush flushes only the calling thread; callers must remain thread-bound
// from drawing through this call and the pixel copy.
func flushCaptureGDI() error {
	if ret, _, _ := procGdiFlush.Call(); ret == 0 {
		return fmt.Errorf("automation: GdiFlush reported a failed drawing operation")
	}
	return nil
}
func makeDIB(w, h int) (uintptr, unsafe.Pointer, func(), error) {
	if w <= 0 || h <= 0 || w > 1<<31-1 || h > 1<<31-1 {
		return 0, nil, func() {}, ErrInvalidRect
	}
	bytes, ok := bitmapByteLen(w, h)
	if !ok || bytes > maxBitmapBytes {
		return 0, nil, func() {}, ErrInvalidRect
	}
	dc, _, callErr := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return 0, nil, func() {}, winCallError(callErr, "CreateCompatibleDC failed")
	}
	bi := bitmapInfo{Header: bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32, Compression: 0}}
	var bits unsafe.Pointer
	hb, _, callErr := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hb == 0 || bits == nil {
		if hb != 0 {
			procDeleteObject.Call(hb)
		}
		procDeleteDC.Call(dc)
		return 0, nil, func() {}, winCallError(callErr, "CreateDIBSection failed")
	}
	old, _, callErr := procSelectObject.Call(dc, hb)
	if old == 0 || old == ^uintptr(0) {
		procDeleteObject.Call(hb)
		procDeleteDC.Call(dc)
		return 0, nil, func() {}, winCallError(callErr, "SelectObject failed")
	}
	cleanup := func() {
		restored, _, _ := procSelectObject.Call(dc, old)
		if restored != 0 && restored != ^uintptr(0) {
			procDeleteObject.Call(hb)
			procDeleteDC.Call(dc)
			return
		}
		// A selected bitmap cannot be deleted. If restoration failed, destroy
		// the memory DC first and then release the no-longer-selected bitmap.
		procDeleteDC.Call(dc)
		procDeleteObject.Call(hb)
	}
	runtime.KeepAlive(bi)
	return dc, bits, cleanup, nil
}
func dibToBitmap(bits unsafe.Pointer, w, h int) (*Bitmap, error) {
	return dibRegionToBitmap(bits, w, h, Rect{0, 0, w, h})
}

func dibRegionToBitmap(bits unsafe.Pointer, sourceWidth, sourceHeight int, region Rect) (*Bitmap, error) {
	if bits == nil {
		return nil, ErrInvalidArgument
	}
	sourceBytes, ok := bitmapByteLen(sourceWidth, sourceHeight)
	if !ok || sourceBytes > maxBitmapBytes {
		return nil, ErrInvalidRect
	}
	if region.Empty() || region.Left < 0 || region.Top < 0 || region.Right > sourceWidth || region.Bottom > sourceHeight {
		return nil, ErrInvalidRect
	}
	b, err := NewBitmap(region.Width(), region.Height())
	if err != nil {
		return nil, err
	}
	src := unsafe.Slice((*byte)(bits), sourceBytes)
	rowBytes := b.Width * 4
	for y := 0; y < b.Height; y++ {
		sourceOffset := ((region.Top+y)*sourceWidth + region.Left) * 4
		destinationOffset := y * rowBytes
		copy(b.Pixels[destinationOffset:destinationOffset+rowBytes], src[sourceOffset:sourceOffset+rowBytes])
	}
	for i := 0; i < len(b.Pixels); i += 4 {
		b.Pixels[i], b.Pixels[i+2] = b.Pixels[i+2], b.Pixels[i]
		b.Pixels[i+3] = 255
	}
	return b, nil
}

func PixelColor(x, y int) (RGB, error) {
	x32, ok := toWinInt32(x)
	if !ok {
		return RGB{}, ErrInvalidArgument
	}
	y32, ok := toWinInt32(y)
	if !ok {
		return RGB{}, ErrInvalidArgument
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hdc, _, callErr := procGetDC.Call(0)
	if hdc == 0 {
		return RGB{}, winCallError(callErr, "GetDC failed")
	}
	defer procReleaseDC.Call(0, hdc)
	v, _, callErr := procGetPixel.Call(hdc, uintptr(x32), uintptr(y32))
	if v == 0xFFFFFFFF {
		return RGB{}, winCallError(callErr, "GetPixel failed")
	}
	return rgbFromColorRef(uint32(v)), nil
}
func rgbFromColorRef(v uint32) RGB { return RGB{uint8(v), uint8(v >> 8), uint8(v >> 16)} }
func PixelColorWindow(hwnd HWND, x, y int, client bool) (RGB, error) {
	if hwnd == 0 {
		return RGB{}, ErrNotFound
	}
	x32, ok := toWinInt32(x)
	if !ok {
		return RGB{}, ErrInvalidArgument
	}
	y32, ok := toWinInt32(y)
	if !ok {
		return RGB{}, ErrInvalidArgument
	}
	p := winPoint{x32, y32}
	if client {
		if ret, _, callErr := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p))); ret == 0 {
			return RGB{}, winCallError(callErr, "ClientToScreen failed")
		}
	}
	return PixelColor(int(p.X), int(p.Y))
}

// CursorPosition reports the current cursor without moving or modifying it.
func CursorPosition() (Point, error) {
	p := winPoint{}
	if ret, _, callErr := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p))); ret == 0 {
		return Point{}, winCallError(callErr, "GetCursorPos failed")
	}
	return Point{int(p.X), int(p.Y)}, nil
}
