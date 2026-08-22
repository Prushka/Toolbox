//go:build windows

package main

import (
	"context"
	"testing"
)

func TestRunInstallsAndRemovesMouseHook(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := run(ctx, []string{"-button", "x2", "-ctrl", "-trigger", "both", "-ignore-injected"}); err != nil {
		t.Fatal(err)
	}
}
