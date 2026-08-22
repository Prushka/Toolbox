package automation

import (
	"errors"
	"testing"
)

func TestInputMonitorOptionValidation(t *testing.T) {
	if got, err := inputEventBufferSize(InputMonitorOptions{}); err != nil || got != defaultInputEventBuffer {
		t.Fatalf("default buffer=%d,%v", got, err)
	}
	if got, err := inputEventBufferSize(InputMonitorOptions{Buffer: 1}); err != nil || got != 1 {
		t.Fatalf("explicit buffer=%d,%v", got, err)
	}
	for _, size := range []int{-1, maxInputEventBuffer + 1} {
		if _, err := inputEventBufferSize(InputMonitorOptions{Buffer: size}); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("buffer %d error=%v", size, err)
		}
	}
}

func TestMouseHotkeyValidation(t *testing.T) {
	valid := MouseHotkey{Button: MouseX2, Modifiers: ModifierControl | ModifierShift, Trigger: MousePressAndRelease}
	if !valid.valid() {
		t.Fatal("valid mouse hotkey rejected")
	}
	for _, binding := range []MouseHotkey{
		{},
		{Button: MouseButton(255)},
		{Button: MousePrimary, Modifiers: Modifiers(0x80)},
		{Button: MousePrimary, Trigger: MouseTrigger(255)},
	} {
		if binding.valid() {
			t.Fatalf("invalid mouse hotkey accepted: %+v", binding)
		}
	}
}
