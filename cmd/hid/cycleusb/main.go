// Command cycleusb demonstrates a physical USB detach/attach and waits for the
// target Leonardo command port to re-enumerate.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Prushka/Toolbox/cmd/hid/internal/example"
	"github.com/Prushka/Toolbox/hid"
)

func main() {
	portName := flag.String("port", "", "Arduino Leonardo CDC port (auto-detected when empty)")
	run := flag.Bool("run", false, "detach and reattach the USB device")
	detachedFor := flag.Duration("duration", time.Second, "USB detached duration (250ms to 30s)")
	waitFor := flag.Duration("wait", 20*time.Second, "maximum re-enumeration wait")
	flag.Parse()
	if !*run {
		log.Fatal("USB cycling is disabled; pass -run to execute the example")
	}
	if *detachedFor < 250*time.Millisecond || *detachedFor > 30*time.Second {
		log.Fatal("-duration must be between 250ms and 30s")
	}
	if *waitFor <= 0 {
		log.Fatal("-wait must be positive")
	}

	commandCtx, cancelCommand := context.WithTimeout(context.Background(), 10*time.Second)
	device, port, err := example.Open(commandCtx, *portName)
	if err != nil {
		cancelCommand()
		log.Fatal(err)
	}
	defer example.Close(device)
	allowRenamedPort := canAcceptRenamedPort(port)
	fmt.Printf("cycling %s for %s\n", port.Name, *detachedFor)
	err = device.CycleUSB(commandCtx, *detachedFor)
	cancelCommand()
	if err != nil {
		log.Fatal(err)
	}

	waitCtx, cancelWait := context.WithTimeout(context.Background(), *waitFor)
	defer cancelWait()
	reconnected, err := waitForPort(waitCtx, port, allowRenamedPort)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("device re-enumerated on %s\n", reconnected.Name)
}

func canAcceptRenamedPort(target hid.Port) bool {
	if target.SerialNumber != "" {
		return true
	}
	ports, err := hid.FindPorts()
	return err == nil && len(ports) == 1 && strings.EqualFold(ports[0].Name, target.Name)
}

func waitForPort(ctx context.Context, target hid.Port, allowRenamedPort bool) (hid.Port, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	sawTargetAbsent := false
	for {
		ports, err := hid.FindPorts()
		if err == nil {
			if current, present := findTarget(target, ports); present {
				if sawTargetAbsent {
					return current, nil
				}
			} else {
				sawTargetAbsent = true
				if allowRenamedPort && target.SerialNumber == "" && len(ports) == 1 {
					return ports[0], nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return hid.Port{}, fmt.Errorf("wait for USB re-enumeration: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func findTarget(target hid.Port, ports []hid.Port) (hid.Port, bool) {
	for _, port := range ports {
		if target.SerialNumber != "" && port.SerialNumber == target.SerialNumber {
			return port, true
		}
		if target.SerialNumber == "" && strings.EqualFold(port.Name, target.Name) {
			return port, true
		}
	}
	return hid.Port{}, false
}
