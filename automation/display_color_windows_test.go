//go:build windows

package automation

import (
	"errors"
	"testing"
	"unsafe"
)

func TestWindowsDisplayColorIdentity(t *testing.T) {
	if got := unsafe.Sizeof(monitorInfo{}); got != 104 {
		t.Fatalf("MONITORINFOEXW size = %d, want 104", got)
	}
	if got := unsafe.Sizeof(displayConfigSourceName{}); got != 84 {
		t.Fatalf("DISPLAYCONFIG_SOURCE_DEVICE_NAME size = %d, want 84", got)
	}
	if got := unsafe.Sizeof(displayConfigAdvancedColorInfo2{}); got != 36 {
		t.Fatalf("DISPLAYCONFIG_GET_ADVANCED_COLOR_INFO_2 size = %d, want 36", got)
	}
	monitors, err := Monitors()
	if err != nil {
		t.Fatal(err)
	}
	colors, err := DisplayColors()
	if errors.Is(err, ErrUnsupported) || errors.Is(err, ErrNotFound) {
		t.Skipf("display color query unavailable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, color := range colors {
		found := false
		for _, monitor := range monitors {
			if color.DeviceName != "" && color.DeviceName == monitor.DeviceName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("display color target %q has no matching monitor in %v", color.DeviceName, monitors)
		}
		t.Logf("display %s: advanced color=%v, mode=%q, SDR white=%d nits", color.DeviceName, color.AdvancedColorEnabled, color.ColorMode, color.SDRWhiteLevelNits)
	}
}
