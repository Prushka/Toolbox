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
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("input failed")
	}
}

func run() error {
	var names example.Strings
	flag.Var(&names, "key", "key or mouse button to poll; repeat for a chord (for example F8 or LButton)")
	interval := flag.Duration("interval", 16*time.Millisecond, "polling interval")
	flag.Parse()
	if *interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	keys := make([]automation.Key, 0, len(names))
	seen := make(map[automation.Key]struct{}, len(names))
	for _, name := range names {
		key, err := automation.ParseKey(name)
		if err != nil {
			return err
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return fmt.Errorf("at least one -key is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	previous, err := automation.PollInput(keys...)
	if err != nil {
		return err
	}
	fmt.Printf("polling %d key(s); press Ctrl+C to stop\n", len(keys))
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
		current, err := automation.PollInput(keys...)
		if err != nil {
			return err
		}
		for _, key := range keys {
			if current.PressedSince(previous, key) {
				fmt.Printf("pressed %s\n", key)
			}
			if current.ReleasedSince(previous, key) {
				fmt.Printf("released %s\n", key)
			}
		}
		previous = current
	}
}
