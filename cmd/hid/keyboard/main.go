// Command keyboard demonstrates chords, held modifiers, text, and special keys
// by opening Notepad and typing into a new document.
package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/Prushka/Toolbox/cmd/hid/internal/example"
	"github.com/Prushka/Toolbox/hid"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("HID keyboard example failed")
	}
}

func run() (err error) {
	portName := flag.String("port", "", "Arduino Leonardo CDC port (auto-detected when empty)")
	activateTitle := flag.String("activate-title", "", "activate the single visible exact-title window before sending input")
	run := flag.Bool("run", false, "send keyboard input")
	keyName := flag.String("key", "", "press one named HID key instead of running the demonstration")
	chord := flag.String("chord", "", "press comma-separated named HID keys as one chord")
	text := flag.String("text", "Typed through the Arduino Leonardo.", "US-ASCII text to type")
	startDelay := flag.Duration("start-delay", time.Second, "delay before sending input")
	tapDelay := flag.Duration("tap-delay", 20*time.Millisecond, "key hold time")
	flag.Parse()
	if !*run {
		return fmt.Errorf("keyboard input is disabled; pass -run to execute the example")
	}
	if *startDelay < 0 {
		return fmt.Errorf("-start-delay cannot be negative")
	}
	if *keyName != "" && *chord != "" {
		return fmt.Errorf("-key and -chord cannot be combined")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, port, err := example.Open(ctx, *portName, hid.WithTapDelay(*tapDelay))
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
	if err := wait(ctx, *startDelay); err != nil {
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
	if *keyName != "" {
		key, err := hid.ParseKey(*keyName)
		if err != nil {
			return err
		}
		if err := device.Do(ctx, hid.TapKeys(key)); err != nil {
			return err
		}
		fmt.Printf("pressed %s through %s\n", *keyName, port.Name)
		return nil
	}
	if *chord != "" {
		keys, err := parseChord(*chord)
		if err != nil {
			return err
		}
		if err := device.Do(ctx, hid.TapKeys(keys...)); err != nil {
			return err
		}
		fmt.Printf("pressed %s through %s\n", *chord, port.Name)
		return nil
	}

	err = device.Do(ctx,
		hid.TapKeys(hid.GUI, hid.MustKey('r')),
		hid.Pause(200*time.Millisecond),
		hid.WriteText("notepad"),
		hid.TapKeys(hid.KeyEnter),
		hid.Pause(750*time.Millisecond),
		hid.HoldKeys(hid.Shift),
		hid.TapKeys(hid.MustKey('t')),
		hid.ReleaseKeys(hid.Shift),
		hid.WriteText("oolbox keyboard example\n"),
		hid.WriteText(*text),
		hid.TapKeys(hid.KeyEnter),
	)
	if err != nil {
		return err
	}
	fmt.Printf("keyboard example completed on %s\n", port.Name)
	return nil
}

func parseChord(value string) ([]hid.Key, error) {
	parts := strings.Split(value, ",")
	keys := make([]hid.Key, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			return nil, fmt.Errorf("keyboard chord contains an empty key")
		}
		key, err := hid.ParseKey(name)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, nil
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
