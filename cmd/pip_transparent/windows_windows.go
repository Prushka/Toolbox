//go:build windows

package main

import (
	"errors"
	"fmt"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	gwlExStyle      int32 = -20
	wsExTransparent       = 0x00000020
	wsExLayered           = 0x00080000

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpNoActivate   = 0x0010
	swpFrameChanged = 0x0020
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	enumWindowsProc         = user32.NewProc("EnumWindows")
	isWindowVisibleProc     = user32.NewProc("IsWindowVisible")
	getWindowTextLengthProc = user32.NewProc("GetWindowTextLengthW")
	getWindowTextProc       = user32.NewProc("GetWindowTextW")
	getPropProc             = user32.NewProc("GetPropW")
	setPropProc             = user32.NewProc("SetPropW")
	removePropProc          = user32.NewProc("RemovePropW")
	setWindowPosProc        = user32.NewProc("SetWindowPos")
	setLastErrorProc        = kernel32.NewProc("SetLastError")
	getLastErrorProc        = kernel32.NewProc("GetLastError")
	getWindowLongPtrProc    = user32.NewProc(windowLongProcName("Get"))
	setWindowLongPtrProc    = user32.NewProc(windowLongProcName("Set"))
	layeredMarkerName       = mustUTF16Ptr("Toolbox.PiPToggle.AddedLayered")
)

type pipWindow struct {
	handle uintptr
	title  string
}

func windowLongProcName(operation string) string {
	if strconv.IntSize == 64 {
		return operation + "WindowLongPtrW"
	}
	return operation + "WindowLongW"
}

func mustUTF16Ptr(value string) *uint16 {
	pointer, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		panic(err)
	}
	return pointer
}

func togglePiPWindows() ([]toggleResult, error) {
	windows, err := findPiPWindows()
	if err != nil {
		return nil, err
	}
	if len(windows) == 0 {
		return nil, errors.New("no visible Picture-in-Picture window found")
	}

	results := make([]toggleResult, 0, len(windows))
	var toggleErrors []error
	for _, window := range windows {
		clickThrough, err := toggleClickThrough(window.handle)
		if err != nil {
			toggleErrors = append(toggleErrors, fmt.Errorf("%q (0x%X): %w", window.title, window.handle, err))
			continue
		}
		results = append(results, toggleResult{
			handle:       window.handle,
			title:        window.title,
			clickThrough: clickThrough,
		})
	}

	return results, errors.Join(toggleErrors...)
}

func findPiPWindows() ([]pipWindow, error) {
	var windows []pipWindow
	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		visible, _, _ := isWindowVisibleProc.Call(hwnd)
		if visible == 0 {
			return 1
		}

		title := windowTitle(hwnd)
		if isPiPTitle(title) {
			windows = append(windows, pipWindow{handle: hwnd, title: title})
		}
		return 1
	})

	setLastErrorProc.Call(0)
	ok, _, _ := enumWindowsProc.Call(callback, 0)
	if ok == 0 {
		return nil, lastError("EnumWindows")
	}
	return windows, nil
}

func windowTitle(hwnd uintptr) string {
	length, _, _ := getWindowTextLengthProc.Call(hwnd)
	if length == 0 {
		return ""
	}

	buffer := make([]uint16, length+1)
	written, _, _ := getWindowTextProc.Call(
		hwnd,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if written == 0 {
		return ""
	}
	return syscall.UTF16ToString(buffer[:written])
}

func toggleClickThrough(hwnd uintptr) (bool, error) {
	style, err := getExtendedStyle(hwnd)
	if err != nil {
		return false, err
	}

	enabling := style&wsExTransparent == 0
	addedLayered := false
	newStyle := style &^ wsExTransparent
	if enabling {
		newStyle = style | wsExTransparent | wsExLayered
		if style&wsExLayered == 0 {
			if err := setLayeredMarker(hwnd); err != nil {
				return false, err
			}
			addedLayered = true
		}
	} else if hasLayeredMarker(hwnd) {
		newStyle &^= wsExLayered
		addedLayered = true
	}

	if err := setExtendedStyle(hwnd, newStyle); err != nil {
		if enabling && addedLayered {
			removeLayeredMarker(hwnd)
		}
		return false, err
	}
	if err := refreshWindowStyle(hwnd); err != nil {
		_ = setExtendedStyle(hwnd, style)
		_ = refreshWindowStyle(hwnd)
		if enabling && addedLayered {
			removeLayeredMarker(hwnd)
		}
		return false, err
	}

	if !enabling && addedLayered {
		removeLayeredMarker(hwnd)
	}
	return enabling, nil
}

func setExtendedStyle(hwnd, style uintptr) error {
	setLastErrorProc.Call(0)
	previousStyle, _, _ := setWindowLongPtrProc.Call(
		hwnd,
		windowLongIndex(),
		style,
	)
	if previousStyle == 0 {
		if code := currentLastError(); code != 0 {
			return fmt.Errorf("SetWindowLong: %w", syscall.Errno(code))
		}
	}
	return nil
}

func refreshWindowStyle(hwnd uintptr) error {
	setLastErrorProc.Call(0)
	ok, _, callErr := setWindowPosProc.Call(
		hwnd,
		0,
		0,
		0,
		0,
		0,
		swpNoSize|swpNoMove|swpNoZOrder|swpNoActivate|swpFrameChanged,
	)
	if ok == 0 {
		if code := currentLastError(); code != 0 {
			return fmt.Errorf("SetWindowPos: %w", syscall.Errno(code))
		}
		return fmt.Errorf("SetWindowPos: %w", callErr)
	}
	return nil
}

func getExtendedStyle(hwnd uintptr) (uintptr, error) {
	setLastErrorProc.Call(0)
	style, _, _ := getWindowLongPtrProc.Call(hwnd, windowLongIndex())
	if style == 0 {
		if code := currentLastError(); code != 0 {
			return 0, fmt.Errorf("GetWindowLong: %w", syscall.Errno(code))
		}
	}
	return style, nil
}

func hasLayeredMarker(hwnd uintptr) bool {
	marker, _, _ := getPropProc.Call(hwnd, uintptr(unsafe.Pointer(layeredMarkerName)))
	return marker != 0
}

func setLayeredMarker(hwnd uintptr) error {
	setLastErrorProc.Call(0)
	ok, _, _ := setPropProc.Call(hwnd, uintptr(unsafe.Pointer(layeredMarkerName)), 1)
	if ok == 0 {
		return lastError("SetPropW")
	}
	return nil
}

func removeLayeredMarker(hwnd uintptr) {
	removePropProc.Call(hwnd, uintptr(unsafe.Pointer(layeredMarkerName)))
}

func windowLongIndex() uintptr {
	index := gwlExStyle
	return uintptr(index)
}

func currentLastError() uintptr {
	code, _, _ := getLastErrorProc.Call()
	return code
}

func lastError(operation string) error {
	if code := currentLastError(); code != 0 {
		return fmt.Errorf("%s: %w", operation, syscall.Errno(code))
	}
	return fmt.Errorf("%s failed", operation)
}
