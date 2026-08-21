//go:build windows

package automation

import "golang.org/x/sys/windows"

var winmm = windows.NewLazySystemDLL("winmm.dll")

func beginTimerResolution(period uint32) (*TimerResolution, error) {
	p := winmm.NewProc("timeBeginPeriod")
	ret, _, _ := p.Call(uintptr(period))
	if ret != 0 {
		return nil, windows.Errno(ret)
	}
	return &TimerResolution{period: period}, nil
}
func endTimerResolution(period uint32) error {
	p := winmm.NewProc("timeEndPeriod")
	ret, _, _ := p.Call(uintptr(period))
	if ret != 0 {
		return windows.Errno(ret)
	}
	return nil
}
