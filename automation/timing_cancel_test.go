package automation

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPreciseSleepCanceledAtShortDurations(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, duration := range []time.Duration{0, time.Nanosecond, time.Microsecond, time.Millisecond, time.Second} {
		if err := PreciseSleep(ctx, duration); !errors.Is(err, context.Canceled) {
			t.Errorf("PreciseSleep(%v) = %v, want canceled", duration, err)
		}
	}
}
