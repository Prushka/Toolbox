package example

import (
	"testing"

	"github.com/Prushka/Toolbox/automation"
)

func TestParseRect(t *testing.T) {
	r, err := ParseRect("-10, 20, 30, 40")
	if err != nil || r != (automation.Rect{Left: -10, Top: 20, Right: 30, Bottom: 40}) {
		t.Fatalf("ParseRect = %+v, %v", r, err)
	}
	if _, err := ParseRect("1,2,3"); err == nil {
		t.Fatal("expected malformed rectangle error")
	}
	if _, err := ParseRect("1,2,1,3"); err == nil {
		t.Fatal("expected empty rectangle error")
	}
}

func TestParseRGBAndTolerance(t *testing.T) {
	c, err := ParseRGB("#12aBef")
	if err != nil || c != (automation.RGB{R: 0x12, G: 0xab, B: 0xef}) {
		t.Fatalf("ParseRGB = %+v, %v", c, err)
	}
	uniform, err := ParseTolerance("7")
	if err != nil || uniform != (automation.ColorTolerance{R: 7, G: 7, B: 7}) {
		t.Fatalf("uniform tolerance = %+v, %v", uniform, err)
	}
	channels, err := ParseTolerance("1, 2, 3")
	if err != nil || channels != (automation.ColorTolerance{R: 1, G: 2, B: 3}) {
		t.Fatalf("channel tolerance = %+v, %v", channels, err)
	}
}

func TestParseCaptureMethod(t *testing.T) {
	for input, want := range map[string]automation.CaptureMethod{
		"visible": automation.CaptureVisible,
		"PRINT":   automation.CapturePrintWindow,
		"auto":    automation.CaptureAuto,
	} {
		got, err := ParseCaptureMethod(input)
		if err != nil || got != want {
			t.Fatalf("ParseCaptureMethod(%q) = %v, %v", input, got, err)
		}
	}
}

func TestInputMonitorFlags(t *testing.T) {
	flags := ModifierFlags{Alt: true, Control: true, Win: true}
	wantModifiers := automation.ModifierAlt | automation.ModifierControl | automation.ModifierWin
	if got := flags.Modifiers(); got != wantModifiers {
		t.Fatalf("modifiers=%v, want %v", got, wantModifiers)
	}
	if got := FormatModifiers(wantModifiers); got != "Ctrl+Alt+Win" {
		t.Fatalf("formatted modifiers=%q", got)
	}
	for input, want := range map[string]automation.MouseButton{
		"PRIMARY":   automation.MousePrimary,
		"secondary": automation.MouseSecondary,
		" middle ":  automation.MouseMiddle,
		"x1":        automation.MouseX1,
		"X2":        automation.MouseX2,
	} {
		got, err := ParseMouseButton(input)
		if err != nil || got != want {
			t.Fatalf("ParseMouseButton(%q)=%v,%v", input, got, err)
		}
		if FormatMouseButton(got) == "" {
			t.Fatalf("FormatMouseButton(%v) is empty", got)
		}
	}
	for input, want := range map[string]automation.MouseTrigger{
		"press":   automation.MousePress,
		"down":    automation.MousePress,
		"release": automation.MouseRelease,
		"up":      automation.MouseRelease,
		"both":    automation.MousePressAndRelease,
	} {
		got, err := ParseMouseTrigger(input)
		if err != nil || got != want {
			t.Fatalf("ParseMouseTrigger(%q)=%v,%v", input, got, err)
		}
	}
	if _, err := ParseMouseButton("wheel"); err == nil {
		t.Fatal("invalid mouse button accepted")
	}
	if got := FormatMouseButton(automation.MouseButton(255)); got != "button-255" {
		t.Fatalf("unknown mouse button=%q", got)
	}
	if _, err := ParseMouseTrigger("sometimes"); err == nil {
		t.Fatal("invalid mouse trigger accepted")
	}
}
