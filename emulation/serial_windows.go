//go:build windows

package emulation

import (
	"context"
	"fmt"

	"go.bug.st/serial"
)

// Open opens a Windows COM port at 115200 baud and verifies that compatible
// firmware is responding. Native USB CDC ignores the nominal baud rate, but a
// conventional value keeps diagnostics predictable.
func Open(ctx context.Context, portName string, options ...Option) (*Client, error) {
	if portName == "" {
		return nil, fmt.Errorf("emulation: COM port name is empty")
	}
	port, err := serial.Open(portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return nil, fmt.Errorf("emulation: open %s: %w", portName, err)
	}
	client, err := NewClient(port, options...)
	if err != nil {
		_ = port.Close()
		return nil, err
	}
	if _, err := client.Info(ctx); err != nil {
		_ = client.shutdown()
		return nil, fmt.Errorf("emulation: handshake on %s: %w", portName, err)
	}
	return client, nil
}
