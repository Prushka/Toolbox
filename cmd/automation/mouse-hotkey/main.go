package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/Prushka/Toolbox/automation"
	"github.com/Prushka/Toolbox/cmd/automation/internal/example"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		log.Fatal().Err(err).Msg("mouse-hotkey failed")
	}
}

func run(ctx context.Context, args []string) (err error) {
	fs := flag.NewFlagSet("mouse-hotkey", flag.ContinueOnError)
	var modifierFlags example.ModifierFlags
	modifierFlags.Bind(fs)
	buttonName := fs.String("button", "primary", "primary, secondary, middle, x1, or x2")
	triggerName := fs.String("trigger", "press", "press, release, or both")
	allowExtra := fs.Bool("allow-extra-modifiers", false, "allow modifiers beyond those requested")
	ignoreInjected := fs.Bool("ignore-injected", false, "ignore events Windows marks as injected")
	buffer := fs.Int("buffer", 64, "event channel capacity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if ctx == nil {
		return automation.ErrInvalidArgument
	}

	button, err := example.ParseMouseButton(*buttonName)
	if err != nil {
		return err
	}
	trigger, err := example.ParseMouseTrigger(*triggerName)
	if err != nil {
		return err
	}
	monitor, err := automation.NewInputMonitor(automation.InputMonitorOptions{Buffer: *buffer})
	if err != nil {
		return err
	}
	defer func() {
		if dropped := monitor.DroppedEvents(); dropped != 0 {
			fmt.Printf("dropped %d event(s) because the consumer was behind\n", dropped)
		}
		if closeErr := monitor.Close(); err == nil {
			err = closeErr
		}
	}()
	binding := automation.MouseHotkey{
		Button:              button,
		Modifiers:           modifierFlags.Modifiers(),
		Trigger:             trigger,
		AllowExtraModifiers: *allowExtra,
		IgnoreInjected:      *ignoreInjected,
	}
	id, err := monitor.RegisterMouse(binding)
	if err != nil {
		return err
	}

	fmt.Println("mouse binding registered; press Ctrl+C to stop")
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-monitor.Events():
			if !ok {
				return monitor.Err()
			}
			if event.Binding == id {
				doSomething(event)
			}
		}
	}
}

// Replace this function body with the work that the mouse binding should trigger.
func doSomething(event automation.InputEvent) {
	edge := "released"
	if event.Kind == automation.EventMousePress {
		edge = "pressed"
	}
	fmt.Printf("%s button %s at %d,%d modifiers=%s injected=%t\n",
		example.FormatMouseButton(event.Button), edge, event.Position.X, event.Position.Y,
		example.FormatModifiers(event.Modifiers), event.Injected)
}
