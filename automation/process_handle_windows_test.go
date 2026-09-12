//go:build windows

package automation

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestPinnedProcessLifecycle(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestPinnedProcessChild$")
	cmd.Env = append(os.Environ(), "TOOLBOX_PROCESS_CHILD=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	p, err := OpenProcess(uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.PID() != uint32(cmd.Process.Pid) || p.Path() == "" {
		t.Fatal("missing process identity")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := p.Terminate(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled termination = %v", err)
	}
	if exited, err := p.Exited(); err != nil || exited {
		t.Fatalf("canceled termination killed child: %t %v", exited, err)
	}
	if err := p.Terminate(t.Context()); err != nil {
		t.Fatal(err)
	}
	waitCtx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	if err := p.Wait(waitCtx); err != nil {
		t.Fatal(err)
	}
	if err := p.Terminate(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}
func TestPinnedProcessChild(t *testing.T) {
	if os.Getenv("TOOLBOX_PROCESS_CHILD") != "1" {
		return
	}
	time.Sleep(time.Hour)
}
