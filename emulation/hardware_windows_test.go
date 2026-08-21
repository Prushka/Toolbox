//go:build hardware && windows

package emulation

import (
	"context"
	"os"
	"testing"
	"time"
)

// This smoke test is opt-in because it talks to a real physical HID device.
// It uses only reversible, low-impact reports: Shift down/up and pointer moves
// that restore the starting position. Protocol/API behavior is covered by the
// transport-backed unit tests without touching the desktop.
func TestLeonardoHardwareSmoke(t *testing.T) {
	if os.Getenv("TOOLBOX_EMULATION_HARDWARE") != "1" {
		t.Skip("set TOOLBOX_EMULATION_HARDWARE=1 to exercise the connected Leonardo")
	}
	portName := os.Getenv("TOOLBOX_EMULATION_PORT")
	if portName == "" {
		ports, err := FindPorts()
		if err != nil {
			t.Fatal(err)
		}
		if len(ports) != 1 {
			t.Fatalf("expected one matching port, found %d", len(ports))
		}
		portName = ports[0].Name
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	device, err := Open(ctx, portName)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()

	info, err := device.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantCapabilities := CapabilityKeyboard | CapabilityRelativeMouse |
		CapabilityAbsoluteMouse | CapabilityHorizontalWheel | CapabilityUSBDetach
	if info.Capabilities&wantCapabilities != wantCapabilities {
		t.Fatalf("capabilities = 0x%04X, missing 0x%04X", info.Capabilities, wantCapabilities)
	}
	if err := device.KeyDown(ctx, Shift); err != nil {
		t.Fatal(err)
	}
	if err := device.KeyUp(ctx, Shift); err != nil {
		t.Fatal(err)
	}
	if err := device.Move(ctx, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := device.Move(ctx, -1, 0); err != nil {
		t.Fatal(err)
	}
	originalX, originalY, err := CursorPosition()
	if err != nil {
		t.Fatal(err)
	}
	targetX, targetY := 100, 100
	if originalX == targetX && originalY == targetY {
		targetX, targetY = 120, 120
	}
	if err := device.MoveTo(ctx, targetX, targetY); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	actualX, actualY, err := CursorPosition()
	if err != nil {
		t.Fatal(err)
	}
	if abs(actualX-targetX) > 3 || abs(actualY-targetY) > 3 {
		_ = device.MoveTo(ctx, originalX, originalY)
		t.Fatalf("MoveTo(%d, %d) landed at (%d, %d)", targetX, targetY, actualX, actualY)
	}
	if err := device.MoveTo(ctx, originalX, originalY); err != nil {
		t.Fatal(err)
	}
	if err := device.ReleaseAll(ctx); err != nil {
		t.Fatal(err)
	}
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
