package automation

import "sync"

// PowerRequestOptions describes the Windows idle-power timers that a caller
// needs suspended while it is doing useful work.
type PowerRequestOptions struct {
	// Reason is shown by Windows in diagnostics such as powercfg /requests.
	Reason string
	// SystemRequired prevents automatic system sleep.
	SystemRequired bool
	// DisplayRequired prevents the display idle timer from turning off.
	DisplayRequired bool
}

// PowerRequest is a process-owned Windows power request. Close clears every
// request established by BeginPowerRequest and is safe to call repeatedly or
// concurrently.
type PowerRequest struct {
	closeOnce sync.Once
	closeFunc func() error
	err       error
}

// BeginPowerRequest asks Windows to suspend the selected idle-power timers.
// The request remains active until Close is called or the process exits.
func BeginPowerRequest(options PowerRequestOptions) (*PowerRequest, error) {
	if options.Reason == "" || (!options.SystemRequired && !options.DisplayRequired) {
		return nil, ErrInvalidArgument
	}
	closeFunc, err := beginPowerRequest(options)
	if err != nil {
		return nil, err
	}
	return &PowerRequest{closeFunc: closeFunc}, nil
}

// Close clears the request and releases its Windows handle.
func (request *PowerRequest) Close() error {
	if request == nil {
		return nil
	}
	request.closeOnce.Do(func() {
		if request.closeFunc != nil {
			request.err = request.closeFunc()
		}
	})
	return request.err
}
