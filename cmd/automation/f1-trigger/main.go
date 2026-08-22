package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Prushka/Toolbox/automation"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatal().Err(err).Msg("F1 trigger failed")
	}
}

func run(ctx context.Context) (err error) {
	if ctx == nil {
		return automation.ErrInvalidArgument
	}
	monitor, err := automation.NewInputMonitor(automation.InputMonitorOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := monitor.Close(); err == nil {
			err = closeErr
		}
	}()
	id, err := monitor.RegisterKeyboard(automation.KeyboardHotkey{Key: automation.KeyF1})
	if err != nil {
		return err
	}

	fmt.Println("Press F1 to run the action; press Ctrl+C to stop")
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-monitor.Events():
			if !ok {
				return monitor.Err()
			}
			if event.Binding == id {
				doSomething()
			}
		}
	}
}

// Replace this function body with the work that F1 should trigger.
func doSomething() {
	fmt.Printf("F1 action ran at %s\n", time.Now().Format(time.RFC3339))
}
