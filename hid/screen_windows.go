//go:build windows

package hid

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32DLL                        = syscall.NewLazyDLL("user32.dll")
	getSystemMetricsProc             = user32DLL.NewProc("GetSystemMetrics")
	getCursorPosProc                 = user32DLL.NewProc("GetCursorPos")
	setThreadDPIAwarenessContextProc = user32DLL.NewProc("SetThreadDpiAwarenessContext")
)

const dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT(-4)

const (
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
)

const pointerSettleTimeout = 5 * time.Second

// Foreground raw-input applications can stop processing pointer state for
// several seconds during a frame stall. Wait through one observed five-second
// stall before deciding that the Windows cursor did not receive an acknowledged
// absolute report.
const absolutePointerSettleTimeout = 7 * time.Second

const absolutePointerReassertDelay = time.Second

const absolutePointerReassertInterval = 250 * time.Millisecond

const windowPointerCalibrationTimeout = 7 * time.Second

const cursorTargetStableDuration = 25 * time.Millisecond

const cursorReportSettleTimeout = 10 * time.Millisecond

// Raw-input consumers may update their in-app pointer a few rendered frames
// after Windows updates the shared cursor. Wait long enough for that pointer
// state to become usable before a caller follows MoveTo with a click.
const rawInputSettleDelay = 350 * time.Millisecond

type cursorPoint struct {
	x int32
	y int32
}

type virtualDesktop struct {
	left   int
	top    int
	width  int
	height int
}

func readVirtualDesktop(metric func(int) int) (virtualDesktop, error) {
	if metric == nil {
		return virtualDesktop{}, errors.New("hid: system metric reader is nil")
	}
	desktop := virtualDesktop{
		left:   metric(smXVirtualScreen),
		top:    metric(smYVirtualScreen),
		width:  metric(smCXVirtualScreen),
		height: metric(smCYVirtualScreen),
	}
	if desktop.width < 2 || desktop.height < 2 {
		return virtualDesktop{}, errors.New("hid: Windows returned invalid virtual display dimensions")
	}
	return desktop, nil
}

func systemMetric(index int) int {
	value, _, _ := getSystemMetricsProc.Call(uintptr(index))
	return int(int32(value))
}

// CursorPosition returns the current Windows cursor position in primary
// display coordinates.
func CursorPosition() (int, int, error) {
	restoreDPI := usePhysicalScreenCoordinates()
	defer restoreDPI()
	return cursorPosition()
}

func cursorPosition() (int, int, error) {
	var point cursorPoint
	ok, _, callErr := getCursorPosProc.Call(uintptr(unsafe.Pointer(&point)))
	if ok == 0 {
		return 0, 0, callErr
	}
	return int(point.x), int(point.y), nil
}

func physicalCursorPosition() (int, int, error) {
	restoreDPI := usePhysicalScreenCoordinates()
	defer restoreDPI()
	return cursorPosition()
}

// MoveTo jumps to a pixel on the Windows primary display using an absolute HID
// report. Use MoveToRelative or MoveToWindow for applications that must observe
// relative movement.
func (client *Client) MoveTo(ctx context.Context, x, y int) error {
	return client.MoveToAbsoluteScreen(ctx, x, y)
}

// ClickAt jumps to a primary-display pixel and clicks using only Arduino HID
// reports. Use ClickAtWindow when a raw-input pointer must also be aligned.
func (client *Client) ClickAt(ctx context.Context, x, y int, buttons ...Button) error {
	if err := client.MoveTo(ctx, x, y); err != nil {
		return err
	}
	return client.Click(ctx, buttons...)
}

// MoveToAbsoluteScreen jumps the Windows cursor to a physical primary-display
// pixel with an absolute Arduino HID report. It may reassert that same
// button-free position after abnormal cursor delay. Raw-input applications may
// ignore absolute reports; use ClickAtWindow when their pointer must also align.
func (client *Client) MoveToAbsoluteScreen(ctx context.Context, x, y int) error {
	if err := client.moveToAbsoluteScreen(ctx, x, y); err != nil {
		return err
	}
	// Absolute reports do not alter a raw-input application's pointer. Preserve
	// that calibrated client position while tracking the Windows cursor so a
	// later MoveToWindow can use a short relative delta.
	client.noteWindowCursorPosition(x, y)
	return nil
}

func (client *Client) moveToAbsoluteScreen(ctx context.Context, x, y int) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	supported, err := client.supports(ctx, CapabilityAbsoluteMouse)
	if err != nil {
		return err
	}
	if !supported {
		return errors.New("hid: firmware does not support absolute mouse input")
	}
	restoreDPI := usePhysicalScreenCoordinates()
	width, _, _ := getSystemMetricsProc.Call(0)
	height, _, _ := getSystemMetricsProc.Call(1)
	restoreDPI()
	if width < 2 || height < 2 {
		return errors.New("hid: Windows returned invalid display dimensions")
	}
	if x < 0 || y < 0 || x >= int(width) || y >= int(height) {
		return errors.New("hid: target is outside the Windows primary display")
	}
	normalizedX := uint16((uint64(x)*32767 + uint64(width-1)/2) / uint64(width-1))
	normalizedY := uint16((uint64(y)*32767 + uint64(height-1)/2) / uint64(height-1))
	deadline := time.Now().Add(absolutePointerSettleTimeout)
	reportCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	reassert := func() error {
		return client.moveAbsoluteReport(reportCtx, normalizedX, normalizedY)
	}
	if err := reassert(); err != nil {
		return err
	}
	return waitForCursorTarget(ctx, "absolute", x, y, deadline, reassert)
}

// MoveToWindow aligns both a foreground raw-input pointer and the Windows
// cursor. screenX/screenY are physical primary-display pixels; clientX/clientY
// are coordinates within the target application's client area. Calibrate
// before entering a placement or drag mode: clamping may temporarily move the
// pointer outside the target window.
func (client *Client) MoveToWindow(ctx context.Context, screenX, screenY, clientX, clientY int) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	if clientX < 0 || clientY < 0 {
		return errors.New("hid: client target cannot be negative")
	}
	// Negotiate before calibration. Otherwise an uninitialized Client falls
	// back to one acknowledged command per report for its first movement.
	if err := client.ensureRelativeMouseSupport(ctx); err != nil {
		return err
	}
	restoreDPI := usePhysicalScreenCoordinates()
	desktop, err := readVirtualDesktop(systemMetric)
	restoreDPI()
	if err != nil {
		return err
	}
	client.windowPointerMu.Lock()
	defer client.windowPointerMu.Unlock()
	currentX, currentY, err := physicalCursorPosition()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	feedbackAvailable := err == nil
	if err != nil && !cursorFeedbackUnavailable(err) {
		return err
	}
	// The last MoveToWindow already calibrated and settled this exact raw-input
	// target. Repeating an absolute report cannot improve its position, and the
	// extra settlement delay only slows a follow-up click at the same point.
	if feedbackAvailable && preparedWindowPointer(client.windowPointer, currentX, currentY, screenX, screenY, clientX, clientY) {
		return nil
	}
	if feedbackAvailable && reusableWindowPointer(client.windowPointer, currentX, currentY, screenX, screenY, clientX, clientY) {
		if err := client.moveWindowTargetReports(ctx, clientX-client.windowPointer.clientX, clientY-client.windowPointer.clientY); err != nil {
			client.windowPointer.valid = false
			return err
		}
		if err := client.moveToAbsoluteScreen(ctx, screenX, screenY); err != nil {
			client.windowPointer.valid = false
			return err
		}
		prepared := windowPointerState{
			valid: true, screenX: screenX, screenY: screenY,
			originX: screenX - clientX, originY: screenY - clientY,
			clientX: clientX, clientY: clientY,
		}
		return settleWindowPointer(ctx, &client.windowPointer, prepared)
	}
	client.windowPointer.valid = false
	// Overshoot the virtual desktop in both directions so a foreground raw-input
	// application clamps its independent pointer to the client edge.
	if err := client.moveRelativeReportsForCalibration(ctx, -desktop.width*2, -desktop.height*2); err != nil {
		return err
	}
	if err := waitForWindowCursorCalibration(ctx, desktop.left, desktop.top); err != nil {
		return err
	}
	if err := client.moveWindowTargetReports(ctx, clientX, clientY); err != nil {
		return err
	}
	// Windows pointer acceleration means this phase's OS endpoint is not the
	// raw-input client coordinate. The firmware has acknowledged every small
	// report; retain a short drain period before absolute OS alignment.
	if err := waitContext(ctx, 100*time.Millisecond); err != nil {
		return err
	}
	if err := client.moveToAbsoluteScreen(ctx, screenX, screenY); err != nil {
		return err
	}
	prepared := windowPointerState{
		valid: true, screenX: screenX, screenY: screenY,
		originX: screenX - clientX, originY: screenY - clientY,
		clientX: clientX, clientY: clientY,
	}
	return settleWindowPointer(ctx, &client.windowPointer, prepared)

}

func settleWindowPointer(ctx context.Context, cached *windowPointerState, prepared windowPointerState) error {
	cached.valid = false
	if err := waitContext(ctx, rawInputSettleDelay); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	*cached = prepared
	return nil
}

// ClickAtWindow aligns both pointer coordinate systems and clicks.
func (client *Client) ClickAtWindow(ctx context.Context, screenX, screenY, clientX, clientY int, buttons ...Button) error {
	for {
		if err := client.MoveToWindow(ctx, screenX, screenY, clientX, clientY); err != nil {
			return err
		}
		err := client.ClickPreparedWindow(ctx, screenX, screenY, clientX, clientY, buttons...)
		if errors.Is(err, ErrWindowPointerMoved) {
			continue
		}
		return err
	}
}

// ClickPreparedWindow clicks only when MoveToWindow most recently prepared the
// exact target and the shared Windows cursor has not since moved. It never
// realigns the pointer, making it safe after a hotkey enters a raw-input mode
// that would be canceled by pointer calibration.
func (client *Client) ClickPreparedWindow(ctx context.Context, screenX, screenY, clientX, clientY int, buttons ...Button) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	if clientX < 0 || clientY < 0 {
		return errors.New("hid: client target cannot be negative")
	}
	client.windowPointerMu.Lock()
	defer client.windowPointerMu.Unlock()
	currentX, currentY, err := physicalCursorPosition()
	if ctxErr := ctx.Err(); ctxErr != nil {
		client.windowPointer.valid = false
		return ctxErr
	}
	if err != nil {
		client.windowPointer.valid = false
		return errors.Join(ErrWindowPointerMoved, err)
	}
	if !preparedWindowPointer(client.windowPointer, currentX, currentY, screenX, screenY, clientX, clientY) {
		client.windowPointer.valid = false
		return ErrWindowPointerMoved
	}
	return client.Click(ctx, buttons...)
}

// MoveToRelative reaches a primary-display pixel using only relative HID
// reports. It is intended for applications that consume raw relative mouse
// input and therefore do not update drag or placement state for absolute HID
// reports. Windows cursor feedback compensates for pointer acceleration.
func (client *Client) MoveToRelative(ctx context.Context, x, y int) error {
	if ctx == nil {
		return errors.New("hid: context is nil")
	}
	client.invalidateWindowPointer()
	restoreDPI := usePhysicalScreenCoordinates()
	defer restoreDPI()

	width, _, _ := getSystemMetricsProc.Call(0)
	height, _, _ := getSystemMetricsProc.Call(1)
	if width < 2 || height < 2 {
		return errors.New("hid: Windows returned invalid display dimensions")
	}
	if x < 0 || y < 0 || x >= int(width) || y >= int(height) {
		return errors.New("hid: target is outside the Windows primary display")
	}
	deadline := time.Now().Add(pointerSettleTimeout)
	moved := false
	for {
		currentX, currentY, err := cursorPosition()
		if err != nil {
			return err
		}
		dx, dy := x-currentX, y-currentY
		if absInt(dx) <= 1 && absInt(dy) <= 1 {
			if !moved {
				return nil
			}
			if err := waitContext(ctx, rawInputSettleDelay); err != nil {
				return err
			}
			settledX, settledY, err := cursorPosition()
			if err != nil {
				return err
			}
			if absInt(x-settledX) <= 1 && absInt(y-settledY) <= 1 {
				return nil
			}
			continue
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("hid: relative pointer did not settle at (%d, %d); current position is (%d, %d)", x, y, currentX, currentY)
		}
		dx = screenDeltaToHID(dx)
		dy = screenDeltaToHID(dy)
		if err := client.moveRelativeReports(ctx, dx, dy); err != nil {
			return err
		}
		moved = true
		if err := waitForCursorChange(ctx, currentX, currentY, deadline); err != nil {
			return err
		}
	}
}

func reusableWindowPointer(cached windowPointerState, currentX, currentY, screenX, screenY, clientX, clientY int) bool {
	if !cached.valid || absInt(currentX-cached.screenX) > 1 || absInt(currentY-cached.screenY) > 1 {
		return false
	}
	return cached.originX == screenX-clientX && cached.originY == screenY-clientY
}

func preparedWindowPointer(cached windowPointerState, currentX, currentY, screenX, screenY, clientX, clientY int) bool {
	return cached.valid &&
		absInt(currentX-cached.screenX) <= 1 && absInt(currentY-cached.screenY) <= 1 &&
		cached.screenX == screenX && cached.screenY == screenY &&
		cached.clientX == clientX && cached.clientY == clientY &&
		cached.originX == screenX-clientX && cached.originY == screenY-clientY
}

func screenDeltaToHID(value int) int {
	direction := 1
	if value < 0 {
		direction = -1
	}
	magnitude := absInt(value)
	step := magnitude
	// Windows pointer acceleration and raw-input consumers transform larger
	// HID deltas differently. Four-count reports are in the measured 1:1
	// range, keeping the OS cursor and application pointer synchronized.
	if step > 4 {
		step = 4
	}
	return direction * step
}

func waitForCursorChange(ctx context.Context, previousX, previousY int, deadline time.Time) error {
	settleDeadline := time.Now().Add(cursorReportSettleTimeout)
	if deadline.Before(settleDeadline) {
		settleDeadline = deadline
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		currentX, currentY, err := cursorPosition()
		if err != nil {
			return err
		}
		if currentX != previousX || currentY != previousY {
			return nil
		}
		if time.Now().After(settleDeadline) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitForWindowCursorCalibration(ctx context.Context, targetX, targetY int) error {
	return waitForCursorTarget(ctx, "window calibration", targetX, targetY, time.Now().Add(windowPointerCalibrationTimeout), nil, true)
}

func waitForCursorTarget(ctx context.Context, kind string, targetX, targetY int, deadline time.Time, reassert func() error, requireFeedback ...bool) error {
	restoreDPI := usePhysicalScreenCoordinates()
	defer restoreDPI()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	required := len(requireFeedback) != 0 && requireFeedback[0]
	return waitForCursorTargetWithFeedback(ctx, kind, targetX, targetY, deadline, cursorPosition, time.Now, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			return nil
		}
	}, reassert, required)
}

func waitForCursorTargetWithFeedback(
	ctx context.Context,
	kind string,
	targetX, targetY int,
	deadline time.Time,
	position func() (int, int, error),
	now func() time.Time,
	poll func(context.Context) error,
	reassert func() error,
	requireFeedback ...bool,
) error {
	required := len(requireFeedback) != 0 && requireFeedback[0]
	var targetObservedAt time.Time
	var nextReassertAt time.Time
	if reassert != nil {
		nextReassertAt = now().Add(absolutePointerReassertDelay)
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		currentX, currentY, err := position()
		if err != nil {
			if cursorFeedbackUnavailable(err) && !required {
				return waitContext(ctx, cursorReportSettleTimeout)
			}
			return err
		}
		// GetCursorPos can complete after a caller deadline (for example while
		// the desktop is stalled). Its result must not authorize success.
		if err := ctx.Err(); err != nil {
			return err
		}
		observedAt := now()
		if err := ctx.Err(); err != nil {
			return err
		}
		if !observedAt.Before(deadline) {
			return fmt.Errorf("hid: %s pointer did not settle at (%d, %d); current position is (%d, %d)", kind, targetX, targetY, currentX, currentY)
		}
		atTarget := absInt(targetX-currentX) <= 1 && absInt(targetY-currentY) <= 1
		if atTarget {
			if targetObservedAt.IsZero() {
				targetObservedAt = observedAt
			}
			if observedAt.Sub(targetObservedAt) >= cursorTargetStableDuration {
				if err := ctx.Err(); err != nil {
					return err
				}
				return nil
			}
		} else {
			targetObservedAt = time.Time{}
		}
		if !atTarget && reassert != nil && !observedAt.Before(nextReassertAt) {
			if err := reassert(); err != nil {
				return err
			}
			nextReassertAt = observedAt.Add(absolutePointerReassertInterval)
		}
		if err := poll(ctx); err != nil {
			return err
		}
	}
}

func cursorFeedbackUnavailable(err error) bool {
	return errors.Is(err, syscall.ERROR_ACCESS_DENIED)
}

func usePhysicalScreenCoordinates() func() {
	if setThreadDPIAwarenessContextProc.Find() != nil {
		return func() {}
	}
	runtime.LockOSThread()
	previous, _, _ := setThreadDPIAwarenessContextProc.Call(dpiAwarenessContextPerMonitorAwareV2)
	if previous == 0 {
		runtime.UnlockOSThread()
		return func() {}
	}
	return func() {
		setThreadDPIAwarenessContextProc.Call(previous)
		runtime.UnlockOSThread()
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
