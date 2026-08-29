//go:build windows

package hid

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"
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

func TestReadVirtualDesktopPreservesNegativeOrigin(t *testing.T) {
	t.Parallel()
	metrics := map[int]int{
		smXVirtualScreen: -1920, smYVirtualScreen: -1080,
		smCXVirtualScreen: 5760, smCYVirtualScreen: 3240,
	}
	desktop, err := readVirtualDesktop(func(index int) int { return metrics[index] })
	if err != nil {
		t.Fatal(err)
	}
	if desktop.left != -1920 || desktop.top != -1080 || desktop.width != 5760 || desktop.height != 3240 {
		t.Fatalf("virtual desktop = %+v", desktop)
	}
}

func TestWaitForCursorTargetRequiresCalibrationFeedback(t *testing.T) {
	t.Parallel()
	start := time.Unix(0, 0)
	err := waitForCursorTargetWithFeedback(
		context.Background(),
		"window calibration",
		0,
		0,
		start.Add(time.Second),
		func() (int, int, error) { return 0, 0, syscall.ERROR_ACCESS_DENIED },
		func() time.Time { return start },
		func(context.Context) error { return nil },
		nil,
		true,
	)
	if !errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
		t.Fatalf("calibration feedback error = %v, want ERROR_ACCESS_DENIED", err)
	}
}

func TestWaitForCursorTargetAllowsFiveSecondFeedbackStall(t *testing.T) {
	t.Parallel()
	start := time.Unix(0, 0)
	current := start
	position := func() (int, int, error) {
		if current.Sub(start) <= 5*time.Second {
			return 10, 20, nil
		}
		return 100, 200, nil
	}

	err := waitForCursorTargetWithFeedback(
		context.Background(),
		"absolute",
		100,
		200,
		start.Add(absolutePointerSettleTimeout),
		position,
		func() time.Time { return current },
		func(context.Context) error {
			current = current.Add(100 * time.Millisecond)
			return nil
		},
		func() error { return nil },
	)
	if err != nil {
		t.Fatalf("waitForCursorTargetWithFeedback error = %v", err)
	}
	if current.Sub(start) <= 5*time.Second {
		t.Fatalf("cursor reached target after %s, want a delay longer than five seconds", current.Sub(start))
	}
}

func TestWaitForCursorTargetReassertsAfterLateRelativeReport(t *testing.T) {
	t.Parallel()
	start := time.Unix(0, 0)
	current := start
	absoluteReports := 1
	position := func() (int, int, error) {
		if current.Equal(start) || absoluteReports > 1 {
			return 100, 200, nil
		}
		return 10, 20, nil
	}

	err := waitForCursorTargetWithFeedback(
		context.Background(),
		"absolute",
		100,
		200,
		start.Add(absolutePointerSettleTimeout),
		position,
		func() time.Time { return current },
		func(context.Context) error {
			current = current.Add(25 * time.Millisecond)
			return nil
		},
		func() error {
			absoluteReports++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("waitForCursorTargetWithFeedback error = %v", err)
	}
	if absoluteReports != 2 {
		t.Fatalf("absolute reports = %d, want 2", absoluteReports)
	}
}

func TestWaitForCursorTargetRejectsCanceledTargetObservation(t *testing.T) {
	t.Parallel()
	start := time.Unix(1, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	position := func() (int, int, error) {
		cancel()
		return 100, 200, nil
	}
	err := waitForCursorTargetWithFeedback(
		ctx,
		"absolute",
		100,
		200,
		start.Add(time.Second),
		position,
		func() time.Time { return start },
		func(context.Context) error { return nil },
		nil,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForCursorTargetWithFeedback error = %v, want context.Canceled", err)
	}
}

func TestWaitForCursorTargetRejectsTargetAtDeadline(t *testing.T) {
	t.Parallel()
	deadline := time.Unix(2, 0)
	err := waitForCursorTargetWithFeedback(
		context.Background(),
		"absolute",
		100,
		200,
		deadline,
		func() (int, int, error) { return 100, 200, nil },
		func() time.Time { return deadline },
		func(context.Context) error { return nil },
		nil,
	)
	if err == nil {
		t.Fatal("waitForCursorTargetWithFeedback succeeded at its deadline")
	}
}

func TestWaitForCursorTargetStopsAtDeadline(t *testing.T) {
	t.Parallel()
	start := time.Unix(0, 0)
	current := start

	err := waitForCursorTargetWithFeedback(
		context.Background(),
		"absolute",
		100,
		200,
		start.Add(absolutePointerSettleTimeout),
		func() (int, int, error) { return 10, 20, nil },
		func() time.Time { return current },
		func(context.Context) error {
			current = current.Add(time.Second)
			return nil
		},
		func() error { return nil },
	)
	if err == nil {
		t.Fatal("waitForCursorTargetWithFeedback succeeded with persistent displacement")
	}
	if current.Sub(start) != absolutePointerSettleTimeout {
		t.Fatalf("wait duration = %s, want %s", current.Sub(start), absolutePointerSettleTimeout)
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
		{"exact cached target", 1200, 900, 1200, 900, 700, 400, true},
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

func TestPreparedWindowPointerRejectsStaleOrigin(t *testing.T) {
	t.Parallel()
	cached := windowPointerState{
		valid: true, screenX: 1200, screenY: 900,
		originX: 499, originY: 500, clientX: 700, clientY: 400,
	}
	if preparedWindowPointer(cached, 1200, 900, 1200, 900, 700, 400) {
		t.Fatal("preparedWindowPointer accepted a stale window origin")
	}
}

func TestSettleWindowPointerDoesNotCacheCanceledPreparation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cached := windowPointerState{valid: true, clientX: 100, clientY: 200}
	prepared := windowPointerState{valid: true, clientX: 300, clientY: 400}

	err := settleWindowPointer(ctx, &cached, prepared)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("settleWindowPointer error = %v, want context.Canceled", err)
	}
	if cached.valid {
		t.Fatal("settleWindowPointer cached a preparation that did not settle")
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
