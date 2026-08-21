//go:build !windows

package automation

type ProcessPriority uint32

const (
	PriorityIdle        ProcessPriority = 0x40
	PriorityBelowNormal                 = 0x4000
	PriorityNormal                      = 0x20
	PriorityAboveNormal                 = 0x8000
	PriorityHigh                        = 0x80
)

func SetProcessPriority(uint32, ProcessPriority) error  { return ErrUnsupported }
func (Window) SetProcessPriority(ProcessPriority) error { return ErrUnsupported }
