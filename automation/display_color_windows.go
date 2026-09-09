//go:build windows

package automation

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	procGetDisplayConfigBufferSizes = user32.NewProc("GetDisplayConfigBufferSizes")
	procQueryDisplayConfig          = user32.NewProc("QueryDisplayConfig")
	procDisplayConfigGetDeviceInfo  = user32.NewProc("DisplayConfigGetDeviceInfo")
)

const (
	qdcOnlyActivePaths                        = 0x00000002
	displayConfigDeviceInfoGetAdvancedColor   = 9
	displayConfigDeviceInfoGetSourceName      = 1
	displayConfigDeviceInfoGetAdvancedColor2  = 15
	displayConfigDeviceInfoGetSDRWhiteLevel   = 11
	displayConfigPathInfoSize                 = 72
	displayConfigModeInfoSize                 = 64
	displayConfigAdvancedColorSupportedBit    = 1 << 0
	displayConfigAdvancedColorEnabledBit      = 1 << 1
	displayConfigWideColorEnforcedBit         = 1 << 2
	displayConfigAdvancedColorForceDisableBit = 1 << 3
)

type displayConfigLUID struct {
	Low  uint32
	High int32
}

type displayConfigDeviceInfoHeader struct {
	Type      uint32
	Size      uint32
	AdapterID displayConfigLUID
	ID        uint32
}

type displayConfigAdvancedColorInfo struct {
	Header              displayConfigDeviceInfoHeader
	Value               uint32
	ColorEncoding       uint32
	BitsPerColorChannel uint32
}

type displayConfigAdvancedColorInfo2 struct {
	Header                                                     displayConfigDeviceInfoHeader
	Value, ColorEncoding, BitsPerColorChannel, ActiveColorMode uint32
}

type displayConfigSDRWhiteLevel struct {
	Header        displayConfigDeviceInfoHeader
	SDRWhiteLevel uint32
}

type displayConfigSourceName struct {
	Header     displayConfigDeviceInfoHeader
	DeviceName [32]uint16
}

// DisplayColorInfo describes the advanced-color (HDR) state of one active
// display target as reported by the Windows display configuration API.
type DisplayColorInfo struct {
	// DeviceName matches Monitor.DeviceName. Mirrored targets can share a name.
	// An empty name means Windows could not identify this display source.
	DeviceName string
	// AdvancedColorSupported reports whether the display can enter HDR or
	// wide-color mode.
	AdvancedColorSupported bool
	// AdvancedColorEnabled reports whether HDR/advanced color is currently on.
	// SDR applications are then tone mapped by the desktop compositor, which
	// can shift the 8-bit values returned by screen capture.
	AdvancedColorEnabled bool
	// ColorMode identifies current composition as "sdr", "wcg", or "hdr" on
	// Windows versions supporting the newer color query. Empty means unknown.
	// AdvancedColorEnabled can remain true in WCG mode with HDR switched off.
	ColorMode string
	// SDRWhiteLevelNits is the brightness Windows assigns to SDR white while
	// advanced color is enabled (80 nits is the reference level). It is zero
	// when the query is unsupported.
	SDRWhiteLevelNits int
	// BitsPerColorChannel is the output depth reported by the driver.
	BitsPerColorChannel int
}

// DisplayColors reports the advanced-color state of every active display
// path. Entries are not ordered by primary status. Match DeviceName with
// Monitors when selecting a specific monitor.
func DisplayColors() ([]DisplayColorInfo, error) {
	if err := procQueryDisplayConfig.Find(); err != nil {
		return nil, ErrUnsupported
	}
	paths, pathCount, err := activeDisplayPaths()
	if err != nil {
		return nil, err
	}
	infos := make([]DisplayColorInfo, 0, pathCount)
	for index := 0; index < int(pathCount); index++ {
		path := paths[index*displayConfigPathInfoSize:]
		// DISPLAYCONFIG_PATH_INFO: source (20 bytes) then target; the target
		// adapter LUID and id are the first 12 bytes of the target block.
		target := path[20:]
		adapter := *(*displayConfigLUID)(unsafe.Pointer(&target[0]))
		id := *(*uint32)(unsafe.Pointer(&target[8]))
		var color displayConfigAdvancedColorInfo
		color.Header = displayConfigDeviceInfoHeader{
			Type: displayConfigDeviceInfoGetAdvancedColor, Size: uint32(unsafe.Sizeof(color)), AdapterID: adapter, ID: id,
		}
		if ret, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&color))); ret != 0 {
			continue
		}
		info := DisplayColorInfo{
			AdvancedColorSupported: color.Value&displayConfigAdvancedColorSupportedBit != 0,
			AdvancedColorEnabled:   color.Value&displayConfigAdvancedColorEnabledBit != 0,
			BitsPerColorChannel:    int(color.BitsPerColorChannel),
		}
		var modern displayConfigAdvancedColorInfo2
		modern.Header = displayConfigDeviceInfoHeader{Type: displayConfigDeviceInfoGetAdvancedColor2,
			Size: uint32(unsafe.Sizeof(modern)), AdapterID: adapter, ID: id}
		if ret, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&modern))); ret == 0 {
			switch modern.ActiveColorMode {
			case 0:
				info.ColorMode = "sdr"
			case 1:
				info.ColorMode = "wcg"
			case 2:
				info.ColorMode = "hdr"
			}
		}
		var name displayConfigSourceName
		name.Header = displayConfigDeviceInfoHeader{
			Type: displayConfigDeviceInfoGetSourceName, Size: uint32(unsafe.Sizeof(name)),
			AdapterID: *(*displayConfigLUID)(unsafe.Pointer(&path[0])), ID: *(*uint32)(unsafe.Pointer(&path[8])),
		}
		if ret, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&name))); ret == 0 {
			info.DeviceName = windows.UTF16ToString(name.DeviceName[:])
		}
		var white displayConfigSDRWhiteLevel
		white.Header = displayConfigDeviceInfoHeader{
			Type: displayConfigDeviceInfoGetSDRWhiteLevel, Size: uint32(unsafe.Sizeof(white)), AdapterID: adapter, ID: id,
		}
		if ret, _, _ := procDisplayConfigGetDeviceInfo.Call(uintptr(unsafe.Pointer(&white))); ret == 0 {
			// The value is in thousandths of the 80-nit reference level.
			info.SDRWhiteLevelNits = int(uint64(white.SDRWhiteLevel) * 80 / 1000)
		}
		infos = append(infos, info)
	}
	if len(infos) == 0 {
		return nil, ErrNotFound
	}
	return infos, nil
}

func activeDisplayPaths() ([]byte, uint32, error) {
	// Topology can change between sizing and querying. Retry fresh sizes, but
	// keep the read bounded if a driver repeatedly changes its answer.
	for range 4 {
		var pathCount, modeCount uint32
		ret, _, _ := procGetDisplayConfigBufferSizes.Call(qdcOnlyActivePaths, uintptr(unsafe.Pointer(&pathCount)), uintptr(unsafe.Pointer(&modeCount)))
		if ret != 0 {
			return nil, 0, fmt.Errorf("automation: GetDisplayConfigBufferSizes failed: %w", windows.Errno(ret))
		}
		if pathCount == 0 {
			return nil, 0, ErrNotFound
		}
		if uint64(pathCount) > uint64(maxInt/displayConfigPathInfoSize) ||
			uint64(modeCount) > uint64(maxInt/displayConfigModeInfoSize) {
			return nil, 0, ErrInvalidArgument
		}
		paths := make([]byte, int(pathCount)*displayConfigPathInfoSize)
		modes := make([]byte, max(int(modeCount), 1)*displayConfigModeInfoSize)
		ret, _, _ = procQueryDisplayConfig.Call(qdcOnlyActivePaths, uintptr(unsafe.Pointer(&pathCount)), uintptr(unsafe.Pointer(&paths[0])), uintptr(unsafe.Pointer(&modeCount)), uintptr(unsafe.Pointer(&modes[0])), 0)
		if ret == uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
			continue
		}
		if ret != 0 {
			return nil, 0, fmt.Errorf("automation: QueryDisplayConfig failed: %w", windows.Errno(ret))
		}
		if uint64(pathCount) > uint64(len(paths)/displayConfigPathInfoSize) {
			return nil, 0, ErrInvalidArgument
		}
		return paths, pathCount, nil
	}
	return nil, 0, fmt.Errorf("automation: display topology did not stabilize: %w", windows.ERROR_INSUFFICIENT_BUFFER)
}
