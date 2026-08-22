package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Prushka/Toolbox/automation"
	"github.com/Prushka/Toolbox/cmd/automation/internal/example"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		log.Fatal().Err(err).Msg("keyboard-hotkey failed")
	}
}

func run(ctx context.Context, args []string) (err error) {
	fs := flag.NewFlagSet("keyboard-hotkey", flag.ContinueOnError)
	var modifierFlags example.ModifierFlags
	modifierFlags.Bind(fs)
	keyName := fs.String("key", "F2", "keyboard key name")
	allowRepeat := fs.Bool("repeat", false, "allow OS key-repeat events while held")
	buffer := fs.Int("buffer", 64, "event channel capacity")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if ctx == nil {
		return automation.ErrInvalidArgument
	}

	key, err := automation.ParseKey(*keyName)
	if err != nil {
		return err
	}
	monitor, err := automation.NewInputMonitor(automation.InputMonitorOptions{Buffer: *buffer})
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := monitor.Close(); err == nil {
			err = closeErr
		}
	}()
	hotkey := automation.KeyboardHotkey{
		Key:         key,
		Modifiers:   modifierFlags.Modifiers(),
		AllowRepeat: *allowRepeat,
	}
	id, err := monitor.RegisterKeyboard(hotkey)
	if err != nil {
		return err
	}

	name := key.String()
	if modifiers := example.FormatModifiers(hotkey.Modifiers); modifiers != "" {
		name = modifiers + "+" + name
	}
	fmt.Printf("registered %s; press Ctrl+C to stop\n", name)
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-monitor.Events():
			if !ok {
				return monitor.Err()
			}
			if event.Binding == id {
				doSomething(name)
			}
		}
	}
}

// Replace this function body with the work that the hotkey should trigger.
func doSomething(name string) {
	fmt.Printf("%s action ran at %s\n", name, time.Now().Format(time.RFC3339))
}
