//go:build windows

package automation

var (
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procGetKeyState      = user32.NewProc("GetKeyState")
)

const smSwapButton = 23

// IsKeyDown reports Windows' current asynchronous state for a keyboard key or
// mouse button. It uses only the high-order GetAsyncKeyState bit. A false result
// does not identify whether the key is up or the interactive desktop is not
// available to this process; Windows reports zero in both cases.
func IsKeyDown(key Key) (bool, error) {
	if !key.Valid() {
		return false, ErrInvalidArgument
	}
	state, _, _ := procGetAsyncKeyState.Call(uintptr(key))
	return uint16(state)&0x8000 != 0, nil
}

// KeyToggleOn reports the toggle state exposed by GetKeyState. It is normally
// used with CapsLock, NumLock, and ScrollLock.
func KeyToggleOn(key Key) (bool, error) {
	if !key.Valid() {
		return false, ErrInvalidArgument
	}
	state, _, _ := procGetKeyState.Call(uintptr(key))
	return uint16(state)&1 != 0, nil
}

// PollInput reads the current physical state of each requested key. Duplicate
// keys are read once. Multi-key snapshots are sampled sequentially because
// Windows provides no atomic GetAsyncKeyState batch operation.
func PollInput(keys ...Key) (InputSnapshot, error) {
	return makeInputSnapshot(keys, func(key Key) bool {
		state, _, _ := procGetAsyncKeyState.Call(uintptr(key))
		return uint16(state)&0x8000 != 0
	})
}

// MouseButtonKey resolves a logical mouse button to the physical virtual key
// used by PollInput and IsKeyDown.
func MouseButtonKey(button MouseButton) (Key, error) {
	switch button {
	case MousePrimary, MouseSecondary:
		swapped, _, _ := procGetSystemMetrics.Call(smSwapButton)
		if (button == MousePrimary) != (swapped == 0) {
			return KeyRButton, nil
		}
		return KeyLButton, nil
	case MouseMiddle:
		return KeyMButton, nil
	case MouseX1:
		return KeyXButton1, nil
	case MouseX2:
		return KeyXButton2, nil
	default:
		return 0, ErrInvalidArgument
	}
}

// MouseButtonDown reports the current state of a logical mouse button.
func MouseButtonDown(button MouseButton) (bool, error) {
	key, err := MouseButtonKey(button)
	if err != nil {
		return false, err
	}
	return IsKeyDown(key)
}
