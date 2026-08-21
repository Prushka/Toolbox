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
	procGetWindowDC        = user32.NewProc("GetWindowDC")
	procGetClientRect      = user32.NewProc("GetClientRect")
	procGetWindowRect      = user32.NewProc("GetWindowRect")
	procClientToScreen     = user32.NewProc("ClientToScreen")
	procScreenToClient     = user32.NewProc("ScreenToClient")
	procPrintWindow        = user32.NewProc("PrintWindow")
	procBitBlt             = gdi32.NewProc("BitBlt")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	procSelectObject       = gdi32.NewProc("SelectObject")
	procDeleteObject       = gdi32.NewProc("DeleteObject")
	procGetPixel           = gdi32.NewProc("GetPixel")
	procGetCursorPos       = user32.NewProc("GetCursorPos")
)

const (
	srcCopy      = 0x00CC0020
	dibRGBColors = 0
	pwClientOnly = 0x00000001
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
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return nil, windows.GetLastError()
	}
	defer procReleaseDC.Call(0, hdc)
	return captureFromDC(hdc, rect.Left, rect.Top, rect.Width(), rect.Height())
}

// CaptureWindow captures a client or full window. The default visible method
// translates bounds to screen coordinates and does not message or open the
// target process.
func CaptureWindow(hwnd HWND, opts CaptureOptions) (*Bitmap, error) {
	if hwnd == 0 {
		return nil, ErrNotFound
	}
	var r winRect
	var ok uintptr
	if opts.ClientOnly {
		ok, _, _ = procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	} else {
		ok, _, _ = procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r)))
	}
	if ok == 0 {
		return nil, windows.GetLastError()
	}
	w, h := int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w <= 0 || h <= 0 {
		return nil, ErrInvalidRect
	}
	var origin winPoint
	if opts.ClientOnly {
		if ret, _, _ := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&origin))); ret == 0 {
			return nil, windows.GetLastError()
		}
	} else {
		origin.X, origin.Y = r.Left, r.Top
	}
	visible := Rect{int(origin.X), int(origin.Y), int(origin.X) + w, int(origin.Y) + h}
	if opts.Method == CaptureVisible {
		return CaptureScreen(visible)
	}
	b, err := capturePrintWindow(hwnd, w, h, opts.ClientOnly)
	if err == nil || opts.Method == CapturePrintWindow {
		return b, err
	}
	return CaptureScreen(visible)
}

// CaptureWindowRegion captures a client-relative region without capturing the
// rest of the window. Regions are clipped to the client area.
func CaptureWindowRegion(hwnd HWND, region Rect, opts CaptureOptions) (*Bitmap, error) {
	if hwnd == 0 {
		return nil, ErrNotFound
	}
	var cr winRect
	if ret, _, _ := procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&cr))); ret == 0 {
		return nil, windows.GetLastError()
	}
	bounds := Rect{0, 0, int(cr.Right - cr.Left), int(cr.Bottom - cr.Top)}
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
	var p winPoint
	if ret, _, _ := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p))); ret == 0 {
		return nil, windows.GetLastError()
	}
	if opts.Method == CaptureVisible {
		return CaptureScreen(Rect{int(p.X) + region.Left, int(p.Y) + region.Top, int(p.X) + region.Right, int(p.Y) + region.Bottom})
	}
	opts.ClientOnly = true
	full, err := CaptureWindow(hwnd, opts)
	if err != nil {
		return nil, err
	}
	return full.Crop(region)
}

type CaptureMethod uint8

const (
	CaptureVisible CaptureMethod = iota
	CapturePrintWindow
	CaptureAuto
)

type CaptureOptions struct {
	ClientOnly bool
	Method     CaptureMethod
}
type winPoint struct{ X, Y int32 }

func capturePrintWindow(hwnd HWND, w, h int, client bool) (*Bitmap, error) {
	dst, bits, cleanup, err := makeDIB(w, h)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := procPrintWindow.Find(); err != nil {
		return nil, err
	}
	flags := uintptr(0)
	if client {
		flags = pwClientOnly
	}
	if ret, _, _ := procPrintWindow.Call(uintptr(hwnd), dst, flags); ret == 0 {
		return nil, windows.GetLastError()
	}
	return dibToBitmap(bits, w, h), nil
}

func captureFromDC(src uintptr, left, top, w, h int) (*Bitmap, error) {
	dst, bits, cleanup, err := makeDIB(w, h)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if ret, _, _ := procBitBlt.Call(dst, 0, 0, uintptr(w), uintptr(h), src, uintptr(left), uintptr(top), srcCopy); ret == 0 {
		return nil, windows.GetLastError()
	}
	return dibToBitmap(bits, w, h), nil
}
func makeDIB(w, h int) (uintptr, unsafe.Pointer, func(), error) {
	if w <= 0 || h <= 0 {
		return 0, nil, func() {}, ErrInvalidRect
	}
	dc, _, _ := procCreateCompatibleDC.Call(0)
	if dc == 0 {
		return 0, nil, func() {}, windows.GetLastError()
	}
	bi := bitmapInfo{Header: bitmapInfoHeader{Size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32, Compression: 0}}
	var bits unsafe.Pointer
	hb, _, _ := procCreateDIBSection.Call(dc, uintptr(unsafe.Pointer(&bi)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hb == 0 || bits == nil {
		procDeleteDC.Call(dc)
		return 0, nil, func() {}, windows.GetLastError()
	}
	old, _, _ := procSelectObject.Call(dc, hb)
	cleanup := func() {
		if old != 0 {
			procSelectObject.Call(dc, old)
		}
		procDeleteObject.Call(hb)
		procDeleteDC.Call(dc)
	}
	runtime.KeepAlive(bi)
	return dc, bits, cleanup, nil
}
func dibToBitmap(bits unsafe.Pointer, w, h int) *Bitmap {
	b, _ := NewBitmap(w, h)
	src := unsafe.Slice((*byte)(bits), w*h*4)
	for i := 0; i < w*h; i++ {
		b.Pixels[i*4], b.Pixels[i*4+1], b.Pixels[i*4+2], b.Pixels[i*4+3] = src[i*4+2], src[i*4+1], src[i*4], 255
	}
	return b
}

func PixelColor(x, y int) (RGB, error) {
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return RGB{}, windows.GetLastError()
	}
	defer procReleaseDC.Call(0, hdc)
	v, _, _ := procGetPixel.Call(hdc, uintptr(x), uintptr(y))
	if v == 0xFFFFFFFF {
		return RGB{}, fmt.Errorf("GetPixel failed")
	}
	return rgbFromColorRef(uint32(v)), nil
}
func rgbFromColorRef(v uint32) RGB { return RGB{uint8(v), uint8(v >> 8), uint8(v >> 16)} }
func PixelColorWindow(hwnd HWND, x, y int, client bool) (RGB, error) {
	p := winPoint{int32(x), int32(y)}
	if client {
		if ret, _, _ := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&p))); ret == 0 {
			return RGB{}, windows.GetLastError()
		}
	}
	return PixelColor(int(p.X), int(p.Y))
}

// CursorPosition reports the current cursor without moving or modifying it.
func CursorPosition() (Point, error) {
	p := winPoint{}
	if ret, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p))); ret == 0 {
		return Point{}, windows.GetLastError()
	}
	return Point{int(p.X), int(p.Y)}, nil
}
