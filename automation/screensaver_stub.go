//go:build !windows

package automation

import "time"

func screenSaverTimeout() (time.Duration, error) { return 0, ErrUnsupported }
func setScreenSaverTimeout(uint32, bool) error   { return ErrUnsupported }
