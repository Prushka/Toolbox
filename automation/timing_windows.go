//go:build windows

package automation

import "golang.org/x/sys/windows"

var (
	winmm               = windows.NewLazySystemDLL("winmm.dll")
	procTimeBeginPeriod = winmm.NewProc("timeBeginPeriod")
	procTimeEndPeriod   = winmm.NewProc("timeEndPeriod")
)

func beginTimerResolution(period uint32) (*TimerResolution, error) {
	ret, _, _ := procTimeBeginPeriod.Call(uintptr(period))
	if ret != 0 {
		return nil, windows.Errno(ret)
	}
	return &TimerResolution{period: period}, nil
}
func endTimerResolution(period uint32) error {
	ret, _, _ := procTimeEndPeriod.Call(uintptr(period))
	if ret != 0 {
		return windows.Errno(ret)
	}
	return nil
}
