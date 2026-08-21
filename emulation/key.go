package emulation

import "fmt"

// Key is an Arduino Keyboard-library key value. Printable keys use their
// lowercase US-ASCII byte; modifiers and navigation keys use the constants
// below.
type Key byte

const (
	KeyLeftCtrl   Key = 0x80
	KeyLeftShift  Key = 0x81
	KeyLeftAlt    Key = 0x82
	KeyLeftGUI    Key = 0x83
	KeyRightCtrl  Key = 0x84
	KeyRightShift Key = 0x85
	KeyRightAlt   Key = 0x86
	KeyRightGUI   Key = 0x87

	KeyEnter     Key = 0xB0
	KeyEscape    Key = 0xB1
	KeyBackspace Key = 0xB2
	KeyTab       Key = 0xB3
	KeyCapsLock  Key = 0xC1
	KeyF1        Key = 0xC2
	KeyF2        Key = 0xC3
	KeyF3        Key = 0xC4
	KeyF4        Key = 0xC5
	KeyF5        Key = 0xC6
	KeyF6        Key = 0xC7
	KeyF7        Key = 0xC8
	KeyF8        Key = 0xC9
	KeyF9        Key = 0xCA
	KeyF10       Key = 0xCB
	KeyF11       Key = 0xCC
	KeyF12       Key = 0xCD
	KeyInsert    Key = 0xD1
	KeyHome      Key = 0xD2
	KeyPageUp    Key = 0xD3
	KeyDelete    Key = 0xD4
	KeyEnd       Key = 0xD5
	KeyPageDown  Key = 0xD6
	KeyRight     Key = 0xD7
	KeyLeft      Key = 0xD8
	KeyDown      Key = 0xD9
	KeyUp        Key = 0xDA
)

// Common modifier aliases make chords concise.
const (
	Ctrl  = KeyLeftCtrl
	Shift = KeyLeftShift
	Alt   = KeyLeftAlt
	GUI   = KeyLeftGUI
)

// RuneKey converts a printable US-ASCII rune to a key. Uppercase letters are
// represented by the same physical key as lowercase letters; use Shift in a
// chord when the case matters. Type handles uppercase text automatically.
func RuneKey(r rune) (Key, error) {
	if r >= 'A' && r <= 'Z' {
		r += 'a' - 'A'
	}
	if r < 0x20 || r > 0x7E {
		return 0, fmt.Errorf("emulation: %q is not a printable US-ASCII key", r)
	}
	return Key(r), nil
}

// MustKey is a convenience for package-level configuration and examples.
func MustKey(r rune) Key {
	key, err := RuneKey(r)
	if err != nil {
		panic(err)
	}
	return key
}
