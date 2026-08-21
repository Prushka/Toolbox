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
	h, _, _ := procOpenProcess.Call(processSetInformation, 0, uintptr(pid))
	if h == 0 {
		return windows.GetLastError()
	}
	defer procCloseHandle.Call(h)
	if ret, _, _ := procSetPriorityClass.Call(h, uintptr(p)); ret == 0 {
		return windows.GetLastError()
	}
	return nil
}
func (w Window) SetProcessPriority(p ProcessPriority) error {
	if w.PID() == 0 {
		return ErrNotFound
	}
	return SetProcessPriority(w.PID(), p)
}
