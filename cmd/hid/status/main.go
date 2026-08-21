// Command status discovers a Leonardo, verifies the command channel, and prints
// the firmware information without generating keyboard or mouse input.
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
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	device, port, err := example.Open(ctx, *portName)
	if err != nil {
		log.Fatal(err)
	}
	defer example.Close(device)

	if err = device.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	info, err := device.Info(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("port: %s\n", port.Name)
	if port.VID != "" {
		fmt.Printf("USB: %s:%s, manufacturer=%q, product=%q, serial=%q\n",
			port.VID, port.PID, port.Manufacturer, port.Product, port.SerialNumber)
	}
	fmt.Printf("firmware: %d.%d\n", info.FirmwareMajor, info.FirmwareMinor)
	fmt.Printf("protocol: %d, maximum payload: %d bytes, watchdog: %s\n",
		info.ProtocolVersion, info.MaximumPayload, info.WatchdogTimeout)
	fmt.Printf("capabilities: 0x%04X (%s)\n", info.Capabilities,
		strings.Join(capabilityNames(info.Capabilities), ", "))
}

func capabilityNames(capabilities hid.Capability) []string {
	known := []struct {
		flag hid.Capability
		name string
	}{
		{hid.CapabilityKeyboard, "keyboard"},
		{hid.CapabilityRelativeMouse, "relative mouse"},
		{hid.CapabilityAbsoluteMouse, "absolute mouse"},
		{hid.CapabilityHorizontalWheel, "horizontal wheel"},
		{hid.CapabilityUSBDetach, "USB detach"},
	}
	names := make([]string, 0, len(known))
	for _, capability := range known {
		if capabilities&capability.flag != 0 {
			names = append(names, capability.name)
		}
	}
	return names
}
