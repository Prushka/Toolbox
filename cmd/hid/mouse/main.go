// Command mouse demonstrates relative/absolute movement, two-axis scrolling,
// clicks, and double-clicks. Potentially disruptive actions are explicit flags.
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/Prushka/Toolbox/cmd/hid/internal/example"
	"github.com/Prushka/Toolbox/hid"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("HID mouse example failed")
	}
}

func run() (err error) {
	portName := flag.String("port", "", "Arduino Leonardo CDC port (auto-detected when empty)")
	activateTitle := flag.String("activate-title", "", "activate the single visible exact-title window before sending input")
	run := flag.Bool("run", false, "send the mouse demonstration")
	dx := flag.Int("dx", 80, "relative horizontal movement")
	dy := flag.Int("dy", 40, "relative vertical movement")
	relativeOnly := flag.Bool("relative-only", false, "send the relative movement once without returning")
	linear := flag.Bool("linear", false, "split relative movement into unaccelerated four-count reports")
	x := flag.Int("x", -1, "optional primary-display absolute X coordinate")
	y := flag.Int("y", -1, "optional primary-display absolute Y coordinate")
	clientX := flag.Int("client-x", -1, "optional foreground raw-input client X for MoveToWindow or ClickAtWindow")
	clientY := flag.Int("client-y", -1, "optional foreground raw-input client Y for MoveToWindow or ClickAtWindow")
	wheel := flag.Int("wheel", 0, "optional vertical scroll amount")
	pan := flag.Int("pan", 0, "optional horizontal scroll amount")
	click := flag.Bool("click", false, "send one left click at the final position")
	buttonName := flag.String("button", "left", "button for -click: left, right, or middle")
	doubleClick := flag.Bool("double-click", false, "send one left double-click at the final position")
	startDelay := flag.Duration("start-delay", 300*time.Millisecond, "delay after activation and device open before sending input")
	pause := flag.Duration("pause", 300*time.Millisecond, "pause between movement steps")
	flag.Parse()
	if !*run {
		return fmt.Errorf("mouse input is disabled; pass -run to execute the example")
	}
	if (*x >= 0) != (*y >= 0) {
		return fmt.Errorf("-x and -y must be provided together")
	}
	if (*clientX >= 0) != (*clientY >= 0) {
		return fmt.Errorf("-client-x and -client-y must be provided together")
	}
	if *clientX >= 0 && (*x < 0 || *doubleClick) {
		return fmt.Errorf("client coordinates require -x and -y without -double-click")
	}
	if *pause < 0 {
		return fmt.Errorf("-pause cannot be negative")
	}
	if *startDelay < 0 {
		return fmt.Errorf("-start-delay cannot be negative")
	}
	button, err := parseButton(*buttonName)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, port, err := example.Open(ctx, *portName)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := example.Close(device); err == nil {
			err = closeErr
		} else if closeErr != nil {
			log.Warn().Err(closeErr).Msg("HID device cleanup failed")
		}
	}()
	if err = wait(ctx, *startDelay); err != nil {
		return err
	}
	if err = example.ActivateWindowByTitle(*activateTitle); err != nil {
		return err
	}
	if *activateTitle != "" {
		if err = wait(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}

	startX, startY, err := hid.CursorPosition()
	if err != nil {
		return err
	}
	fmt.Printf("starting cursor position: (%d, %d)\n", startX, startY)
	if *clientX >= 0 {
		operation := "window move"
		if *click {
			operation = "window click"
			err = device.ClickAtWindow(ctx, *x, *y, *clientX, *clientY, button)
		} else {
			err = device.MoveToWindow(ctx, *x, *y, *clientX, *clientY)
		}
		if err != nil {
			return err
		}
		endX, endY, positionErr := hid.CursorPosition()
		if positionErr != nil {
			return positionErr
		}
		fmt.Printf("%s completed on %s; final cursor position: (%d, %d)\n", operation, port.Name, endX, endY)
		return nil
	}

	move := device.Move
	if *linear {
		move = device.MoveLinear
	}
	if err = move(ctx, *dx, *dy); err != nil {
		return err
	}
	if !*relativeOnly {
		if err = wait(ctx, *pause); err != nil {
			return err
		}
		if err = move(ctx, -*dx, -*dy); err != nil {
			return err
		}
	}
	if *x >= 0 {
		if err = device.MoveTo(ctx, *x, *y); err != nil {
			return err
		}
		if err = wait(ctx, *pause); err != nil {
			return err
		}
	}
	if *wheel != 0 || *pan != 0 {
		if err = device.Scroll(ctx, *wheel, *pan); err != nil {
			return err
		}
	}
	if *click {
		if err = device.Click(ctx, button); err != nil {
			return err
		}
	}
	if *doubleClick {
		if err = device.DoubleClick(ctx, 100*time.Millisecond); err != nil {
			return err
		}
	}
	endX, endY, err := hid.CursorPosition()
	if err != nil {
		return err
	}
	fmt.Printf("mouse example completed on %s; final cursor position: (%d, %d)\n", port.Name, endX, endY)
	return nil
}

func parseButton(name string) (hid.Button, error) {
	switch name {
	case "left":
		return hid.ButtonLeft, nil
	case "right":
		return hid.ButtonRight, nil
	case "middle":
		return hid.ButtonMiddle, nil
	default:
		return 0, fmt.Errorf("unsupported mouse button %q", name)
	}
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
