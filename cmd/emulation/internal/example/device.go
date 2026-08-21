// Package example contains shared setup used by the runnable emulation examples.
package example

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Prushka/Toolbox/emulation"
)

// ResolvePort uses an explicit COM port or requires exactly one matching board.
func ResolvePort(requested string) (emulation.Port, error) {
	if requested != "" {
		ports, err := emulation.FindPorts()
		if err == nil {
			for _, port := range ports {
				if strings.EqualFold(port.Name, requested) {
					return port, nil
				}
			}
		}
		return emulation.Port{Name: requested}, nil
	}
	ports, err := emulation.FindPorts()
	if err != nil {
		return emulation.Port{}, err
	}
	return SelectPort(ports)
}

// SelectPort requires exactly one discovered board and provides useful errors.
func SelectPort(ports []emulation.Port) (emulation.Port, error) {
	if len(ports) == 1 {
		return ports[0], nil
	}
	names := make([]string, len(ports))
	for index, port := range ports {
		names[index] = port.Name
	}
	detail := "none"
	if len(names) > 0 {
		detail = strings.Join(names, ", ")
	}
	return emulation.Port{}, fmt.Errorf(
		"expected exactly one %s:%s command port, found %d (%s); pass -port explicitly",
		emulation.LogitechVendorID, emulation.LogitechProductID, len(ports), detail,
	)
}

// Open resolves a port and performs the firmware handshake.
func Open(ctx context.Context, requested string, options ...emulation.Option) (*emulation.Client, emulation.Port, error) {
	port, err := ResolvePort(requested)
	if err != nil {
		return nil, emulation.Port{}, err
	}
	device, err := emulation.Open(ctx, port.Name, options...)
	if err != nil {
		return nil, emulation.Port{}, err
	}
	return device, port, nil
}

// Close reports cleanup failures without hiding the example's primary result.
func Close(device *emulation.Client) {
	if err := device.Close(); err != nil {
		log.Printf("close device: %v", err)
	}
}
