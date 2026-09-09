//go:build windows

package automation

import (
	"context"
	"errors"
	"testing"
)

func TestEnsureActiveRejectsCanceledContextBeforeWindowAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// Even an absent window must return cancellation before native access.
	if err := (Window{}).EnsureActive(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("EnsureActive = %v, want cancellation", err)
	}
	active, err := ActiveWindow()
	if err != nil {
		t.Skipf("no foreground window: %v", err)
	}
	if err := active.EnsureActive(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("already-active EnsureActive = %v, want cancellation", err)
	}
}
