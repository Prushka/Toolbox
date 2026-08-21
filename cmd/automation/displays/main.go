package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Prushka/Toolbox/automation"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("displays failed")
	}
}

func run() error {
	showModes := flag.Bool("modes", false, "list available primary-display modes")
	limit := flag.Int("limit", 30, "maximum modes to print; zero prints all")
	flag.Parse()
	if *limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if err := automation.SetDPIAware(); err != nil && !errors.Is(err, automation.ErrUnsupported) {
		return err
	}

	virtual := automation.ScreenRect()
	primary := automation.PrimaryScreenRect()
	current, err := automation.CurrentDisplayMode()
	if err != nil {
		return err
	}
	monitors, err := automation.Monitors()
	if err != nil {
		return err
	}
	fmt.Printf("virtual=%s primary=%s current=%s\n", formatRect(virtual), formatRect(primary), formatMode(current))
	for index, monitor := range monitors {
		fmt.Printf("monitor[%d] rect=%s work=%s primary=%t\n",
			index, formatRect(monitor.Rect), formatRect(monitor.WorkArea), monitor.Primary)
	}
	if !*showModes {
		return nil
	}
	modes, err := automation.DisplayModes()
	if err != nil {
		return err
	}
	printed := len(modes)
	if *limit > 0 && printed > *limit {
		printed = *limit
	}
	for _, mode := range modes[:printed] {
		fmt.Println(formatMode(mode))
	}
	if printed < len(modes) {
		fmt.Printf("... %d more modes (increase -limit)\n", len(modes)-printed)
	}
	return nil
}

func formatRect(r automation.Rect) string {
	return fmt.Sprintf("%d,%d,%d,%d", r.Left, r.Top, r.Right, r.Bottom)
}

func formatMode(mode automation.DisplayMode) string {
	return fmt.Sprintf("%dx%d %dbpp %dHz", mode.Width, mode.Height, mode.BitsPerPixel, mode.Frequency)
}
