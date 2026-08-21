//go:build windows

package automation

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procEnumDisplaySettings   = user32.NewProc("EnumDisplaySettingsW")
	procChangeDisplaySettings = user32.NewProc("ChangeDisplaySettingsW")
	procEnumDisplayMonitors   = user32.NewProc("EnumDisplayMonitors")
	procGetMonitorInfo        = user32.NewProc("GetMonitorInfoW")
)

const (
	enumCurrentSettings  = ^uint32(0)
	dmBitsPerPel         = 0x00040000
	dmPelsWidth          = 0x00080000
	dmPelsHeight         = 0x00100000
	dmDisplayFrequency   = 0x00400000
	cdsUpdateRegistry    = 0x00000001
	dispChangeSuccessful = 0
	monitorInfoPrimary   = 1
)

// devMode is DEVMODEW. Its size is checked in the Windows integration tests.
type devMode struct {
	DeviceName                                                                                     [32]uint16
	SpecVersion, DriverVersion, Size, DriverExtra                                                  uint16
	Fields                                                                                         uint32
	DisplayUnion                                                                                   [16]byte
	Color, Duplex, YResolution, TTOption, Collate                                                  uint16
	FormName                                                                                       [32]uint16
	LogPixels                                                                                      uint16
	BitsPerPel, PelsWidth, PelsHeight, DisplayFlags, DisplayFrequency                              uint32
	ICMMethod, ICMIntent, MediaType, DitherType, Reserved1, Reserved2, PanningWidth, PanningHeight uint32
}

type DisplayMode struct{ Width, Height, BitsPerPixel, Frequency int }

func CurrentDisplayMode() (DisplayMode, error) {
	var dm devMode
	dm.Size = uint16(unsafe.Sizeof(dm))
	if ret, _, _ := procEnumDisplaySettings.Call(0, uintptr(enumCurrentSettings), uintptr(unsafe.Pointer(&dm))); ret == 0 {
		return DisplayMode{}, windows.GetLastError()
	}
	return modeFromDev(dm), nil
}
func DisplayModes() ([]DisplayMode, error) {
	var out []DisplayMode
	seen := map[DisplayMode]bool{}
	for i := uint32(0); ; i++ {
		var dm devMode
		dm.Size = uint16(unsafe.Sizeof(dm))
		ret, _, _ := procEnumDisplaySettings.Call(0, uintptr(i), uintptr(unsafe.Pointer(&dm)))
		if ret == 0 {
			break
		}
		m := modeFromDev(dm)
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return nil, ErrNotFound
	}
	return out, nil
}
func modeFromDev(d devMode) DisplayMode {
	return DisplayMode{int(d.PelsWidth), int(d.PelsHeight), int(d.BitsPerPel), int(d.DisplayFrequency)}
}

// SetDisplayMode changes the primary display. Permanent changes persist
// through restart; temporary changes last for the current session.
func SetDisplayMode(m DisplayMode, permanent bool) error {
	if m.Width <= 0 || m.Height <= 0 {
		return ErrInvalidRect
	}
	var dm devMode
	dm.Size = uint16(unsafe.Sizeof(dm))
	dm.PelsWidth, dm.PelsHeight = uint32(m.Width), uint32(m.Height)
	dm.Fields = dmPelsWidth | dmPelsHeight
	if m.BitsPerPixel > 0 {
		dm.BitsPerPel = uint32(m.BitsPerPixel)
		dm.Fields |= dmBitsPerPel
	}
	if m.Frequency > 0 {
		dm.DisplayFrequency = uint32(m.Frequency)
		dm.Fields |= dmDisplayFrequency
	}
	flags := uintptr(0)
	if permanent {
		flags = cdsUpdateRegistry
	}
	ret, _, _ := procChangeDisplaySettings.Call(uintptr(unsafe.Pointer(&dm)), flags)
	if int32(ret) != dispChangeSuccessful {
		return fmt.Errorf("automation: display mode change failed with code %d", int32(ret))
	}
	return nil
}
func RestoreDisplayMode() error {
	ret, _, _ := procChangeDisplaySettings.Call(0, 0)
	if int32(ret) != dispChangeSuccessful {
		return fmt.Errorf("automation: display restore failed with code %d", int32(ret))
	}
	return nil
}

type Monitor struct {
	Rect, WorkArea Rect
	Primary        bool
}
type monitorInfo struct {
	Size          uint32
	Monitor, Work winRect
	Flags         uint32
}

func Monitors() ([]Monitor, error) {
	var out []Monitor
	cb := windows.NewCallback(func(hmon, hdc, rect, data uintptr) uintptr {
		mi := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if ok, _, _ := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi))); ok != 0 {
			out = append(out, Monitor{Rect: Rect{int(mi.Monitor.Left), int(mi.Monitor.Top), int(mi.Monitor.Right), int(mi.Monitor.Bottom)}, WorkArea: Rect{int(mi.Work.Left), int(mi.Work.Top), int(mi.Work.Right), int(mi.Work.Bottom)}, Primary: mi.Flags&monitorInfoPrimary != 0})
		}
		return 1
	})
	if ret, _, _ := procEnumDisplayMonitors.Call(0, 0, cb, 0); ret == 0 {
		return nil, windows.GetLastError()
	}
	return out, nil
}
