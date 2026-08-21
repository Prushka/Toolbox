package automation

import (
	"context"
	"math/rand"
	"runtime"
	"sync"
	"time"
)

var processStarted = time.Now()
var jitterRandMu sync.Mutex

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
	if ctx == nil {
		return ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return err
	}
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
	if ctx == nil {
		return ErrInvalidArgument
	}
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
	if ctx == nil {
		return ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if minus < 0 {
		minus = 0
	}
	if plus < 0 {
		plus = 0
	}
	maxDuration := time.Duration(1<<63 - 1)
	lo := time.Duration(0)
	if d > 0 && minus < d {
		lo = d - minus
	}
	if minus > maxDuration-plus {
		return ErrInvalidArgument
	}
	span := minus + plus
	if lo > maxDuration-span {
		return ErrInvalidArgument
	}
	delta := time.Duration(0)
	if span > 0 {
		if span == maxDuration {
			if r == nil {
				delta = time.Duration(rand.Int63())
			} else {
				jitterRandMu.Lock()
				delta = time.Duration(r.Int63())
				jitterRandMu.Unlock()
			}
		} else if r == nil {
			delta = time.Duration(rand.Int63n(int64(span) + 1))
		} else {
			jitterRandMu.Lock()
			delta = time.Duration(r.Int63n(int64(span) + 1))
			jitterRandMu.Unlock()
		}
	}
	if lo > maxDuration-delta {
		return ErrInvalidArgument
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
