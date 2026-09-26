package automation

import (
	"errors"
	"testing"
	"time"
)

func TestScreenSaverSeconds(t *testing.T) {
	for _, value := range []time.Duration{0, -time.Second, time.Millisecond, time.Second + 1, (1 << 32) * time.Second} {
		if _, err := screenSaverSeconds(value); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("timeout %v: %v", value, err)
		}
	}
	for _, value := range []time.Duration{time.Second, 10 * time.Minute, 100000 * time.Minute, (1<<32 - 1) * time.Second} {
		seconds, err := screenSaverSeconds(value)
		if err != nil || time.Duration(seconds)*time.Second != value {
			t.Fatalf("timeout %v: %d, %v", value, seconds, err)
		}
	}
}
