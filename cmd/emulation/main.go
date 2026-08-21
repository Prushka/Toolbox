//go:build windows

// Example command-line client for the Arduino Leonardo HID emulator.
//
// Usage:
//
//	go run ./cmd/emulation
//	go run ./cmd/emulation -demo
//
// The example intentionally performs only a visible, reversible demo: it
// opens Notepad via the keyboard, types a short marker, then releases all
// inputs before exiting. Remove the demo actions and use the emulation package
// from your own program for real workflows.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Prushka/Toolbox/emulation"
)

func main() {
	port := flag.String("port", "", "Arduino Leonardo CDC port (auto-detected when empty)")
	demo := flag.Bool("demo", false, "run the Notepad keyboard demo")
	cycle := flag.Duration("cycle", 0, "detach and reattach USB for this duration, then exit")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if *port == "" {
		ports, findErr := emulation.FindPorts()
		if findErr != nil {
			log.Fatal(findErr)
		}
		if len(ports) != 1 {
			log.Fatalf("expected exactly one %s:%s command port, found %d; pass -port explicitly",
				emulation.LogitechVendorID, emulation.LogitechProductID, len(ports))
		}
		*port = ports[0].Name
	}

	device, err := emulation.Open(ctx, *port)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := device.Close(); closeErr != nil {
			log.Printf("close device: %v", closeErr)
		}
	}()
	info, err := device.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: firmware %d.%d, protocol %d, capabilities 0x%04X, watchdog %s\n",
		*port,
		info.FirmwareMajor, info.FirmwareMinor, info.ProtocolVersion,
		info.Capabilities, info.WatchdogTimeout)
	if *cycle != 0 {
		if err = device.CycleUSB(ctx, *cycle); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("USB cycle requested for %s; reconnect after Windows enumerates the device\n", *cycle)
		return
	}
	if !*demo {
		return
	}

	if err = device.Do(ctx,
		emulation.TapKeys(emulation.GUI, emulation.MustKey('r')),
		emulation.Pause(150*time.Millisecond),
		emulation.WriteText("notepad"),
		emulation.TapKeys(emulation.KeyEnter),
		emulation.Pause(500*time.Millisecond),
		emulation.WriteText("Toolbox Leonardo HID demo"),
	); err != nil {
		log.Fatal(err)
	}
	fmt.Println("demo completed; all inputs will be released on exit")
}
