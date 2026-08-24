//go:build windows

package hid

import (
	"errors"
	"syscall"
	"testing"
)

func TestCursorFeedbackUnavailable(t *testing.T) {
	t.Parallel()
	if !cursorFeedbackUnavailable(syscall.ERROR_ACCESS_DENIED) {
		t.Fatal("ERROR_ACCESS_DENIED was not treated as unavailable cursor feedback")
	}
	if !cursorFeedbackUnavailable(errors.Join(errors.New("cursor read"), syscall.ERROR_ACCESS_DENIED)) {
		t.Fatal("wrapped ERROR_ACCESS_DENIED was not treated as unavailable cursor feedback")
	}
	if cursorFeedbackUnavailable(syscall.Errno(6)) {
		t.Fatal("ERROR_INVALID_HANDLE was treated as unavailable cursor feedback")
	}
}

func TestResetWindowPointer(t *testing.T) {
	t.Parallel()
	client := &Client{windowPointer: windowPointerState{valid: true, clientX: 100, clientY: 200}}
	client.ResetWindowPointer()
	if client.windowPointer.valid {
		t.Fatal("ResetWindowPointer retained cached calibration")
	}
}

func TestReusableWindowPointer(t *testing.T) {
	t.Parallel()
	cached := windowPointerState{
		valid: true, screenX: 1200, screenY: 900,
		originX: 500, originY: 500, clientX: 700, clientY: 400,
	}
	for _, test := range []struct {
		name                               string
		currentX, currentY                 int
		screenX, screenY, clientX, clientY int
		want                               bool
	}{
		{"same origin", 1200, 900, 1300, 950, 800, 450, true},
		{"cursor tolerance", 1201, 899, 1300, 950, 800, 450, true},
		{"physical mouse moved", 1210, 900, 1300, 950, 800, 450, false},
		{"window origin moved", 1200, 900, 1400, 950, 800, 450, false},
		{"invalid", 1200, 900, 1300, 950, 800, 450, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := cached
			if test.name == "invalid" {
				state.valid = false
			}
			got := reusableWindowPointer(state, test.currentX, test.currentY,
				test.screenX, test.screenY, test.clientX, test.clientY)
			if got != test.want {
				t.Fatalf("reusableWindowPointer() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestPreparedWindowPointer(t *testing.T) {
	t.Parallel()
	cached := windowPointerState{
		valid: true, screenX: 1200, screenY: 900,
		originX: 500, originY: 500, clientX: 700, clientY: 400,
	}
	for _, test := range []struct {
		name                               string
		currentX, currentY                 int
		screenX, screenY, clientX, clientY int
		want                               bool
	}{
		{"exact", 1200, 900, 1200, 900, 700, 400, true},
		{"cursor tolerance", 1201, 899, 1200, 900, 700, 400, true},
		{"physical mouse moved", 1210, 900, 1200, 900, 700, 400, false},
		{"different screen target", 1200, 900, 1300, 950, 700, 400, false},
		{"different client target", 1200, 900, 1200, 900, 800, 450, false},
		{"invalid", 1200, 900, 1200, 900, 700, 400, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := cached
			if test.name == "invalid" {
				state.valid = false
			}
			got := preparedWindowPointer(state, test.currentX, test.currentY,
				test.screenX, test.screenY, test.clientX, test.clientY)
			if got != test.want {
				t.Fatalf("preparedWindowPointer() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestClampLinearDelta(t *testing.T) {
	t.Parallel()
	for input, want := range map[int]int{-10: -4, -4: -4, -3: -3, 0: 0, 3: 3, 4: 4, 10: 4} {
		if got := clampLinearDelta(input); got != want {
			t.Errorf("clampLinearDelta(%d) = %d, want %d", input, got, want)
		}
	}
}
