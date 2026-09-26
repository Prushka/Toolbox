package automation

import "time"

// ScreenSaverTimeout returns the current Windows screen saver idle timeout.
// It does not report the monitor sleep timeout or change screen saver activation.
func ScreenSaverTimeout() (time.Duration, error) { return screenSaverTimeout() }

// SetScreenSaverTimeout changes the screen saver idle timeout. A positive whole
// number of seconds fitting a Windows UINT is required. With persist=false the
// change applies only to the current Windows session; persist=true also updates
// the user profile. Callers own synchronization and restoration after failure.
// Windows may wrap very large values; read back and verify the required timeout.
// This does not change the screen saver, its enabled state, or password policy.
func SetScreenSaverTimeout(timeout time.Duration, persist bool) error {
	seconds, err := screenSaverSeconds(timeout)
	if err != nil {
		return err
	}
	return setScreenSaverTimeout(seconds, persist)
}

func screenSaverSeconds(timeout time.Duration) (uint32, error) {
	if timeout <= 0 || timeout%time.Second != 0 || timeout/time.Second > 1<<32-1 {
		return 0, ErrInvalidArgument
	}
	return uint32(timeout / time.Second), nil
}
