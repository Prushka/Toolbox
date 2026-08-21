package automation

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

var processStarted = time.Now()

// TickCount returns milliseconds elapsed since this package was initialized,
// equivalent to the monotonic portion of AHK's A_TickCount for process-local
// timeouts.
func TickCount() int64              { return time.Since(processStarted).Milliseconds() }
func ElapsedSince(last int64) int64 { return TickCount() - last }
func FormatTimestamp(t time.Time, layout string) string {
	if layout == "" {
		layout = "20060102-150405"
	}
	return t.Format(layout)
}

// Sleep is a context-aware replacement for AHK Sleep. It returns nil when the
// duration elapsed, or ctx.Err when canceled.
func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// PreciseSleep uses a short final spin to reduce scheduler overshoot while
// retaining a bounded CPU cost. It does not change system timer resolution.
func PreciseSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return Sleep(ctx, d)
	}
	deadline := time.Now().Add(d)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil
		}
		if remaining > 2*time.Millisecond {
			if err := Sleep(ctx, remaining-time.Millisecond); err != nil {
				return err
			}
		} else {
			runtimeYield()
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
	}
}

// JitterSleep sleeps in [d-minus,d+plus], useful when a caller wants the
// randomized timing behavior of the Genshin scripts without hidden globals.
func JitterSleep(ctx context.Context, d, minus, plus time.Duration, r *rand.Rand) error {
	if minus < 0 {
		minus = 0
	}
	if plus < 0 {
		plus = 0
	}
	lo := d - minus
	if lo < 0 {
		lo = 0
	}
	span := minus + plus
	delta := time.Duration(0)
	if span > 0 {
		if r == nil {
			r = rand.New(rand.NewSource(time.Now().UnixNano()))
		}
		delta = time.Duration(r.Int63n(int64(span) + 1))
	}
	return Sleep(ctx, lo+delta)
}

// TimerResolution requests the Windows multimedia timer period. On other
// platforms it is a no-op. Close restores the prior period.
type TimerResolution struct {
	period uint32
	once   sync.Once
	err    error
}

func BeginTimerResolution(period uint32) (*TimerResolution, error) {
	if period == 0 {
		period = 1
	}
	return beginTimerResolution(period)
}
func (t *TimerResolution) Close() error {
	if t == nil {
		return nil
	}
	t.once.Do(func() { t.err = endTimerResolution(t.period) })
	return t.err
}

func runtimeYield() { runtime.Gosched() }
