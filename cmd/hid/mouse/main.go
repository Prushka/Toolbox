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
	run := flag.Bool("run", false, "send the mouse demonstration")
	dx := flag.Int("dx", 80, "relative horizontal movement")
	dy := flag.Int("dy", 40, "relative vertical movement")
	x := flag.Int("x", -1, "optional primary-display absolute X coordinate")
	y := flag.Int("y", -1, "optional primary-display absolute Y coordinate")
	wheel := flag.Int("wheel", 0, "optional vertical scroll amount")
	pan := flag.Int("pan", 0, "optional horizontal scroll amount")
	click := flag.Bool("click", false, "send one left click at the final position")
	doubleClick := flag.Bool("double-click", false, "send one left double-click at the final position")
	pause := flag.Duration("pause", 300*time.Millisecond, "pause between movement steps")
	flag.Parse()
	if !*run {
		return fmt.Errorf("mouse input is disabled; pass -run to execute the example")
	}
	if (*x >= 0) != (*y >= 0) {
		return fmt.Errorf("-x and -y must be provided together")
	}
	if *pause < 0 {
		return fmt.Errorf("-pause cannot be negative")
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

	startX, startY, err := hid.CursorPosition()
	if err != nil {
		return err
	}
	fmt.Printf("starting cursor position: (%d, %d)\n", startX, startY)

	if err = device.Move(ctx, *dx, *dy); err != nil {
		return err
	}
	if err = wait(ctx, *pause); err != nil {
		return err
	}
	if err = device.Move(ctx, -*dx, -*dy); err != nil {
		return err
	}
	if *x >= 0 {
		if err = device.MoveTo(ctx, *x, *y); err != nil {
			return err
		}
		if err = wait(ctx, *pause); err != nil {
			return err
		}
		if err = device.MoveTo(ctx, startX, startY); err != nil {
			log.Warn().Err(err).Int("x", startX).Int("y", startY).
				Msg("could not restore cursor with primary-display absolute movement")
		}
	}
	if *wheel != 0 || *pan != 0 {
		if err = device.Scroll(ctx, *wheel, *pan); err != nil {
			return err
		}
	}
	if *click {
		if err = device.Click(ctx, hid.ButtonLeft); err != nil {
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
