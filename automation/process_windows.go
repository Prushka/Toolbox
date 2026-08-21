//go:build windows

package automation

import "golang.org/x/sys/windows"

var procSetPriorityClass = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetPriorityClass")

const processSetInformation = 0x0200

type ProcessPriority uint32

const (
	PriorityIdle        ProcessPriority = 0x40
	PriorityBelowNormal                 = 0x4000
	PriorityNormal                      = 0x20
	PriorityAboveNormal                 = 0x8000
	PriorityHigh                        = 0x80
)

func SetProcessPriority(pid uint32, p ProcessPriority) error {
	if pid == 0 || !validProcessPriority(p) {
		return ErrInvalidArgument
	}
	h, _, callErr := procOpenProcess.Call(processSetInformation, 0, uintptr(pid))
	if h == 0 {
		return winCallError(callErr, "OpenProcess failed")
	}
	defer procCloseHandle.Call(h)
	if ret, _, callErr := procSetPriorityClass.Call(h, uintptr(p)); ret == 0 {
		return winCallError(callErr, "SetPriorityClass failed")
	}
	return nil
}
func (w Window) SetProcessPriority(p ProcessPriority) error {
	pid := w.PID()
	if pid == 0 {
		return ErrNotFound
	}
	return SetProcessPriority(pid, p)
}

func validProcessPriority(p ProcessPriority) bool {
	switch p {
	case PriorityIdle, PriorityBelowNormal, PriorityNormal, PriorityAboveNormal, PriorityHigh:
		return true
	default:
		return false
	}
}
