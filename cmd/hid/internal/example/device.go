// Package example contains shared setup used by the runnable HID examples.
package example

import (
	"context"
	"fmt"
	"strings"

	"github.com/Prushka/Toolbox/hid"
)

// ResolvePort uses an explicit COM port or requires exactly one matching board.
func ResolvePort(requested string) (hid.Port, error) {
	if requested != "" {
		ports, err := hid.FindPorts()
		if err == nil {
			for _, port := range ports {
				if strings.EqualFold(port.Name, requested) {
					return port, nil
				}
			}
		}
		return hid.Port{Name: requested}, nil
	}
	ports, err := hid.FindPorts()
	if err != nil {
		return hid.Port{}, err
	}
	return SelectPort(ports)
}

// SelectPort requires exactly one discovered board and provides useful errors.
func SelectPort(ports []hid.Port) (hid.Port, error) {
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
	return hid.Port{}, fmt.Errorf(
		"expected exactly one %s:%s command port, found %d (%s); pass -port explicitly",
		hid.LogitechVendorID, hid.LogitechProductID, len(ports), detail,
	)
}

// Open resolves a port and performs the firmware handshake.
func Open(ctx context.Context, requested string, options ...hid.Option) (*hid.Client, hid.Port, error) {
	port, err := ResolvePort(requested)
	if err != nil {
		return nil, hid.Port{}, err
	}
	device, err := hid.Open(ctx, port.Name, options...)
	if err != nil {
		return nil, hid.Port{}, err
	}
	return device, port, nil
}

// Close releases input and closes the command port.
func Close(device *hid.Client) error {
	if device == nil {
		return nil
	}
	return device.Close()
}
