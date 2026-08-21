//go:build !windows

package emulation

const (
	LogitechVendorID  = "046D"
	LogitechProductID = "C223"
)

type Port struct {
	Name         string
	VID          string
	PID          string
	SerialNumber string
	Manufacturer string
	Product      string
}

func FindPorts() ([]Port, error) {
	return nil, ErrUnsupported
}
