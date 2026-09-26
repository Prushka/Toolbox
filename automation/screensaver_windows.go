//go:build windows

package automation

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var screenSaverSystemParametersInfo = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

func screenSaverTimeout() (time.Duration, error) {
	var seconds uint32
	ok, _, err := screenSaverSystemParametersInfo.Call(0x000E, 0, uintptr(unsafe.Pointer(&seconds)), 0)
	if ok == 0 {
		return 0, screenSaverError("get", err)
	}
	return time.Duration(seconds) * time.Second, nil
}

func setScreenSaverTimeout(seconds uint32, persist bool) error {
	var flags uintptr
	if persist {
		flags = 1 | 2 // SPIF_UPDATEINIFILE | SPIF_SENDCHANGE
	}
	ok, _, err := screenSaverSystemParametersInfo.Call(0x000F, uintptr(seconds), 0, flags)
	if ok == 0 {
		return screenSaverError("set", err)
	}
	return nil
}

func screenSaverError(operation string, err error) error {
	if err == nil || err == windows.ERROR_SUCCESS {
		err = windows.ERROR_GEN_FAILURE
	}
	return fmt.Errorf("automation: screen saver timeout %s: %w", operation, err)
}
