//go:build windows

package automation

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	powerRequestContextVersion      = 0
	powerRequestContextSimpleString = 1
	powerRequestDisplayRequired     = 0
	powerRequestSystemRequired      = 1
)

var (
	powerCreateRequest = windows.NewLazySystemDLL("kernel32.dll").NewProc("PowerCreateRequest")
	powerSetRequest    = windows.NewLazySystemDLL("kernel32.dll").NewProc("PowerSetRequest")
	powerClearRequest  = windows.NewLazySystemDLL("kernel32.dll").NewProc("PowerClearRequest")
)

type powerRequestContext struct {
	Version            uint32
	Flags              uint32
	SimpleReasonString *uint16
	// REASON_CONTEXT contains a union whose detailed member is larger than
	// the simple string pointer. Reserve its complete native ABI storage.
	localizedReasonID uint32
	reasonStringCount uint32
	reasonStrings     uintptr
}

func beginPowerRequest(options PowerRequestOptions) (func() error, error) {
	reason, err := windows.UTF16PtrFromString(options.Reason)
	if err != nil {
		return nil, fmt.Errorf("automation: encode power request reason: %w", err)
	}
	requestContext := powerRequestContext{
		Version:            powerRequestContextVersion,
		Flags:              powerRequestContextSimpleString,
		SimpleReasonString: reason,
	}
	handleValue, _, callErr := powerCreateRequest.Call(uintptr(unsafe.Pointer(&requestContext)))
	runtime.KeepAlive(reason)
	if handleValue == uintptr(windows.InvalidHandle) {
		return nil, powerRequestCallError("create", callErr)
	}
	handle := windows.Handle(handleValue)

	types := make([]uintptr, 0, 2)
	if options.SystemRequired {
		types = append(types, powerRequestSystemRequired)
	}
	if options.DisplayRequired {
		types = append(types, powerRequestDisplayRequired)
	}
	enabled := make([]uintptr, 0, len(types))
	for _, requestType := range types {
		result, _, setErr := powerSetRequest.Call(handleValue, requestType)
		if result == 0 {
			cleanupErr := clearPowerRequests(handle, enabled)
			return nil, errors.Join(powerRequestCallError("set", setErr), cleanupErr)
		}
		enabled = append(enabled, requestType)
	}

	return func() error {
		return clearPowerRequests(handle, enabled)
	}, nil
}

func clearPowerRequests(handle windows.Handle, enabled []uintptr) error {
	var result error
	for index := len(enabled) - 1; index >= 0; index-- {
		cleared, _, callErr := powerClearRequest.Call(uintptr(handle), enabled[index])
		if cleared == 0 {
			result = errors.Join(result, powerRequestCallError("clear", callErr))
		}
	}
	return errors.Join(result, windows.CloseHandle(handle))
}

func powerRequestCallError(operation string, callErr error) error {
	if callErr == nil || errors.Is(callErr, syscall.Errno(0)) {
		callErr = windows.ERROR_GEN_FAILURE
	}
	return fmt.Errorf("automation: power request %s: %w", operation, callErr)
}
