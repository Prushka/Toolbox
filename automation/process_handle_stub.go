//go:build !windows

package automation

import "context"

type Process struct{}

func (Window) IsHung() bool { return false }

func OpenProcess(uint32) (*Process, error)       { return nil, ErrUnsupported }
func (*Process) PID() uint32                     { return 0 }
func (*Process) Path() string                    { return "" }
func (*Process) Close() error                    { return ErrUnsupported }
func (*Process) Exited() (bool, error)           { return false, ErrUnsupported }
func (*Process) Wait(context.Context) error      { return ErrUnsupported }
func (*Process) Terminate(context.Context) error { return ErrUnsupported }
