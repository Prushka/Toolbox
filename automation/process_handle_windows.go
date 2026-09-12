//go:build windows

package automation

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// Process pins a Windows process object so termination cannot target a reused
// PID. Close releases the handle; it does not terminate the process.
type Process struct {
	mu     sync.Mutex
	handle windows.Handle
	pid    uint32
	path   string
}

var procIsHungAppWindow = windows.NewLazySystemDLL("user32.dll").NewProc("IsHungAppWindow")

// IsHung reports Windows' message-pump hang assessment. A false result does
// not establish rendering progress; GPU rendering may stall independently.
func (w Window) IsHung() bool {
	if !w.Valid() {
		return false
	}
	result, _, _ := procIsHungAppWindow.Call(uintptr(w.Handle()))
	return result != 0
}

func OpenProcess(pid uint32) (*Process, error) {
	if pid == 0 {
		return nil, ErrInvalidArgument
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return nil, err
	}
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(h, 0, &buffer[0], &size); err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	return &Process{handle: h, pid: pid, path: windows.UTF16ToString(buffer[:size])}, nil
}

func (p *Process) PID() uint32 {
	if p == nil {
		return 0
	}
	return p.pid
}
func (p *Process) Path() string {
	if p == nil {
		return ""
	}
	return p.path
}
func (p *Process) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(p.handle)
	if err == nil {
		p.handle = 0
	}
	return err
}
func (p *Process) Exited() (bool, error) {
	if p == nil {
		return false, ErrInvalidArgument
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return false, ErrInvalidArgument
	}
	state, err := windows.WaitForSingleObject(p.handle, 0)
	return state == windows.WAIT_OBJECT_0, err
}
func (p *Process) Wait(ctx context.Context) error {
	if ctx == nil {
		return ErrInvalidArgument
	}
	return WaitUntil(ctx, 50*time.Millisecond, func() (bool, error) { return p.Exited() })
}

// Terminate forcefully ends only the pinned process. Callers must establish
// their own application-specific authorization before calling it.
func (p *Process) Terminate(ctx context.Context) error {
	if ctx == nil || p == nil {
		return ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return ErrInvalidArgument
	}
	state, err := windows.WaitForSingleObject(p.handle, 0)
	if err != nil {
		return err
	}
	if state == windows.WAIT_OBJECT_0 {
		return nil
	}
	return windows.TerminateProcess(p.handle, 1)
}
