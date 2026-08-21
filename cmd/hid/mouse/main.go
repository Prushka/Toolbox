// Command mouse demonstrates relative/absolute movement, two-axis scrolling,
// clicks, and double-clicks. Potentially disruptive actions are explicit flags.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Prushka/Toolbox/cmd/hid/internal/example"
	"github.com/Prushka/Toolbox/hid"
)

func main() {
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
		log.Fatal("mouse input is disabled; pass -run to execute the example")
	}
	if (*x >= 0) != (*y >= 0) {
		log.Fatal("-x and -y must be provided together")
	}
	if *pause < 0 {
		log.Fatal("-pause cannot be negative")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, port, err := example.Open(ctx, *portName)
	if err != nil {
		log.Fatal(err)
	}
	defer example.Close(device)

	startX, startY, err := hid.CursorPosition()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("starting cursor position: (%d, %d)\n", startX, startY)

	if err = device.Move(ctx, *dx, *dy); err != nil {
		log.Fatal(err)
	}
	if err = wait(ctx, *pause); err != nil {
		log.Fatal(err)
	}
	if err = device.Move(ctx, -*dx, -*dy); err != nil {
		log.Fatal(err)
	}
	if *x >= 0 {
		if err = device.MoveTo(ctx, *x, *y); err != nil {
			log.Fatal(err)
		}
		if err = wait(ctx, *pause); err != nil {
			log.Fatal(err)
		}
		if err = device.MoveTo(ctx, startX, startY); err != nil {
			log.Printf("could not restore (%d, %d) with primary-display absolute movement: %v", startX, startY, err)
		}
	}
	if *wheel != 0 || *pan != 0 {
		if err = device.Scroll(ctx, *wheel, *pan); err != nil {
			log.Fatal(err)
		}
	}
	if *click {
		if err = device.Click(ctx, hid.ButtonLeft); err != nil {
			log.Fatal(err)
		}
	}
	if *doubleClick {
		if err = device.DoubleClick(ctx, 100*time.Millisecond); err != nil {
			log.Fatal(err)
		}
	}
	endX, endY, err := hid.CursorPosition()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("mouse example completed on %s; final cursor position: (%d, %d)\n", port.Name, endX, endY)
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
