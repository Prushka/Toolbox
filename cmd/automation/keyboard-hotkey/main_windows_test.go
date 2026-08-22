//go:build windows

package main

import (
	"context"
	"testing"
)

func TestRunRegistersAndClosesKeyboardHotkey(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := run(ctx, []string{"-key", "F23", "-ctrl", "-alt", "-shift"}); err != nil {
		t.Fatal(err)
	}
}
