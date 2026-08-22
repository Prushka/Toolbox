package main

import (
	"errors"
	"testing"

	"github.com/Prushka/Toolbox/automation"
)

func TestRunRejectsInvalidConfigurationBeforeMonitoring(t *testing.T) {
	if err := run(t.Context(), []string{"-button", "wheel"}); err == nil {
		t.Fatal("invalid button accepted")
	}
	if err := run(t.Context(), []string{"-trigger", "sometimes"}); err == nil {
		t.Fatal("invalid trigger accepted")
	}
	if err := run(t.Context(), []string{"-buffer", "-1"}); !errors.Is(err, automation.ErrInvalidArgument) {
		t.Fatalf("invalid buffer error=%v", err)
	}
	if err := run(nil, nil); !errors.Is(err, automation.ErrInvalidArgument) {
		t.Fatalf("nil context error=%v", err)
	}
}
