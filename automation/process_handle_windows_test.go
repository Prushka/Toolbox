//go:build windows

package automation

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/windows"
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
	if err := windows.TerminateProcess(p.handle, 1); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("observation handle unexpectedly permits termination: %v", err)
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

func TestPinnedTerminationRejectsDifferentProcessObject(t *testing.T) {
	var children []*exec.Cmd
	for range 2 {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPinnedProcessChild$")
		cmd.Env = append(os.Environ(), "TOOLBOX_PROCESS_CHILD=1")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
		children = append(children, cmd)
	}
	p, err := OpenProcess(uint32(children[0].Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	other, err := OpenProcess(uint32(children[1].Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	// Model PID reuse: the lookup now identifies another live process, while
	// the original kernel object remains pinned. Neither may be terminated.
	p.pid = other.PID()
	if err := p.Terminate(t.Context()); err == nil {
		t.Fatal("different kernel object was accepted")
	}
	for _, process := range []*Process{p, other} {
		if exited, err := process.Exited(); err != nil || exited {
			t.Fatalf("process was not preserved: exited=%t error=%v", exited, err)
		}
	}
}
