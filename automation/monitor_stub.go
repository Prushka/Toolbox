//go:build !windows

package automation

// InputMonitor is unavailable outside Windows.
type InputMonitor struct{}

func NewInputMonitor(options InputMonitorOptions) (*InputMonitor, error) {
	if _, err := inputEventBufferSize(options); err != nil {
		return nil, err
	}
	return nil, ErrUnsupported
}
func (*InputMonitor) RegisterKeyboard(KeyboardHotkey) (BindingID, error) {
	return 0, ErrUnsupported
}
func (*InputMonitor) RegisterMouse(MouseHotkey) (BindingID, error) { return 0, ErrUnsupported }
func (*InputMonitor) Unregister(BindingID) error                   { return ErrUnsupported }
func (*InputMonitor) Events() <-chan InputEvent                    { return nil }
func (*InputMonitor) Done() <-chan struct{}                        { return nil }
func (*InputMonitor) DroppedEvents() uint64                        { return 0 }
func (*InputMonitor) Err() error                                   { return ErrUnsupported }
func (*InputMonitor) Close() error                                 { return ErrUnsupported }
