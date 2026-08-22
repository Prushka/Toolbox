//go:build !windows

package automation

func IsKeyDown(Key) (bool, error)               { return false, ErrUnsupported }
func KeyToggleOn(Key) (bool, error)             { return false, ErrUnsupported }
func PollInput(...Key) (InputSnapshot, error)   { return InputSnapshot{}, ErrUnsupported }
func MouseButtonKey(MouseButton) (Key, error)   { return 0, ErrUnsupported }
func MouseButtonDown(MouseButton) (bool, error) { return false, ErrUnsupported }
