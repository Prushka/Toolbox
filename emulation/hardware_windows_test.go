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
	if info.FirmwareMajor != 1 || info.FirmwareMinor < 1 {
		t.Fatalf("firmware version = %d.%d, want at least 1.1", info.FirmwareMajor, info.FirmwareMinor)
	}
	wantCapabilities := CapabilityKeyboard | CapabilityRelativeMouse |
		CapabilityAbsoluteMouse | CapabilityHorizontalWheel | CapabilityUSBDetach
	if info.Capabilities&wantCapabilities != wantCapabilities {
		t.Fatalf("capabilities = 0x%04X, missing 0x%04X", info.Capabilities, wantCapabilities)
	}

	corrupt, err := encodeFrame(requestMagic, 0xF0, byte(opKeyDown), []byte{'x'})
	if err != nil {
		t.Fatal(err)
	}
	corrupt[len(corrupt)-1] ^= 0xFF
	if err = writeAll(device.transport, corrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-device.responses:
		if result.err != nil || result.frame.sequence != 0xF0 ||
			status(result.frame.code) != statusBadChecksum {
			t.Fatalf("bad-checksum response = %#v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("firmware did not reject corrupt frame")
	}

	// The parser must discard an incomplete request instead of treating the
	// next valid command as the tail of the abandoned frame.
	if err = writeAll(device.transport, []byte{requestMagic, protocolVersion, 0xF1}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err = device.Ping(ctx); err != nil {
		t.Fatalf("Ping after partial-frame expiry: %v", err)
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
	deadline := time.Now().Add(500 * time.Millisecond)
	var actualX, actualY int
	for {
		actualX, actualY, err = CursorPosition()
		if err != nil {
			t.Fatal(err)
		}
		if abs(actualX-targetX) <= 5 && abs(actualY-targetY) <= 5 {
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if abs(actualX-targetX) > 5 || abs(actualY-targetY) > 5 {
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
