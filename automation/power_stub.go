//go:build !windows

package automation

func beginPowerRequest(PowerRequestOptions) (func() error, error) {
	return nil, ErrUnsupported
}
