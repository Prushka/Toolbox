//go:build windows

package hid

import (
	"context"
	"errors"
	"testing"
)

func TestOpenRejectsCanceledContextBeforePortAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Open(ctx, "not-a-real-COM-port"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Open = %v, want context cancellation before port access", err)
	}
	if _, err := Open(nil, "not-a-real-COM-port"); err == nil || err.Error() != "hid: context is nil" {
		t.Fatalf("nil-context Open = %v", err)
	}
}
