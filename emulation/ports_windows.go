//go:build windows

package emulation

import (
	"fmt"
	"strings"

	"go.bug.st/serial/enumerator"
)

const (
	LogitechVendorID  = "046D"
	LogitechProductID = "C223"
)

// Port describes a matching Leonardo CDC command port.
type Port struct {
	Name         string
	VID          string
	PID          string
	SerialNumber string
	Manufacturer string
	Product      string
}

// FindPorts returns USB serial ports with the firmware's Logitech VID/PID.
// It does not open a port or generate any keyboard/mouse input.
func FindPorts() ([]Port, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("emulation: enumerate serial ports: %w", err)
	}
	var ports []Port
	for _, detail := range details {
		if !detail.IsUSB || !strings.EqualFold(detail.VID, LogitechVendorID) ||
			!strings.EqualFold(detail.PID, LogitechProductID) {
			continue
		}
		ports = append(ports, Port{
			Name:         detail.Name,
			VID:          strings.ToUpper(detail.VID),
			PID:          strings.ToUpper(detail.PID),
			SerialNumber: detail.SerialNumber,
			Manufacturer: detail.Manufacturer,
			Product:      detail.Product,
		})
	}
	return ports, nil
}
