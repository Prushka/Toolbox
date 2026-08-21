//go:build windows

package hid

import (
	"context"
	"errors"
	"syscall"
	"unsafe"
)

var (
	user32DLL            = syscall.NewLazyDLL("user32.dll")
	getSystemMetricsProc = user32DLL.NewProc("GetSystemMetrics")
	getCursorPosProc     = user32DLL.NewProc("GetCursorPos")
)

type cursorPoint struct {
	x int32
	y int32
}

// CursorPosition returns the current Windows cursor position in primary
// display coordinates.
func CursorPosition() (int, int, error) {
	var point cursorPoint
	ok, _, callErr := getCursorPosProc.Call(uintptr(unsafe.Pointer(&point)))
	if ok == 0 {
		return 0, 0, callErr
	}
	return int(point.x), int(point.y), nil
}

// MoveTo jumps the hardware pointer to a pixel on the Windows primary display.
func (client *Client) MoveTo(ctx context.Context, x, y int) error {
	width, _, _ := getSystemMetricsProc.Call(0)
	height, _, _ := getSystemMetricsProc.Call(1)
	if width < 2 || height < 2 {
		return errors.New("hid: Windows returned invalid display dimensions")
	}
	if x < 0 || y < 0 || x >= int(width) || y >= int(height) {
		return errors.New("hid: target is outside the Windows primary display")
	}
	normalizedX := uint16((uint64(x)*32767 + uint64(width-1)/2) / uint64(width-1))
	normalizedY := uint16((uint64(y)*32767 + uint64(height-1)/2) / uint64(height-1))
	return client.MoveAbsolute(ctx, normalizedX, normalizedY)
}
