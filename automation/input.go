package automation

import (
	"fmt"
	"strconv"
	"strings"
)

// Key is a Windows virtual-key code. Mouse buttons are represented by the
// KeyLButton through KeyXButton2 values and can be polled with keyboard keys.
type Key uint8

// Valid reports whether k is in the range accepted by GetAsyncKeyState.
func (k Key) Valid() bool { return k >= 0x01 && k <= 0xfe }

// Windows virtual-key codes. The short names follow common AutoHotkey names;
// aliases below provide longer Go-friendly spellings where useful.
const (
	KeyLButton          Key = 0x01
	KeyRButton          Key = 0x02
	KeyCancel           Key = 0x03
	KeyMButton          Key = 0x04
	KeyXButton1         Key = 0x05
	KeyXButton2         Key = 0x06
	KeyBackspace        Key = 0x08
	KeyTab              Key = 0x09
	KeyClear            Key = 0x0c
	KeyEnter            Key = 0x0d
	KeyShift            Key = 0x10
	KeyCtrl             Key = 0x11
	KeyAlt              Key = 0x12
	KeyPause            Key = 0x13
	KeyCapsLock         Key = 0x14
	KeyKana             Key = 0x15
	KeyIMEOn            Key = 0x16
	KeyJunja            Key = 0x17
	KeyFinal            Key = 0x18
	KeyHanja            Key = 0x19
	KeyIMEOff           Key = 0x1a
	KeyEsc              Key = 0x1b
	KeyConvert          Key = 0x1c
	KeyNonConvert       Key = 0x1d
	KeyAccept           Key = 0x1e
	KeyModeChange       Key = 0x1f
	KeySpace            Key = 0x20
	KeyPgUp             Key = 0x21
	KeyPgDn             Key = 0x22
	KeyEnd              Key = 0x23
	KeyHome             Key = 0x24
	KeyLeft             Key = 0x25
	KeyUp               Key = 0x26
	KeyRight            Key = 0x27
	KeyDown             Key = 0x28
	KeySelect           Key = 0x29
	KeyPrint            Key = 0x2a
	KeyExecute          Key = 0x2b
	KeyPrintScreen      Key = 0x2c
	KeyInsert           Key = 0x2d
	KeyDelete           Key = 0x2e
	KeyHelp             Key = 0x2f
	Key0                Key = 0x30
	Key1                Key = 0x31
	Key2                Key = 0x32
	Key3                Key = 0x33
	Key4                Key = 0x34
	Key5                Key = 0x35
	Key6                Key = 0x36
	Key7                Key = 0x37
	Key8                Key = 0x38
	Key9                Key = 0x39
	KeyA                Key = 0x41
	KeyB                Key = 0x42
	KeyC                Key = 0x43
	KeyD                Key = 0x44
	KeyE                Key = 0x45
	KeyF                Key = 0x46
	KeyG                Key = 0x47
	KeyH                Key = 0x48
	KeyI                Key = 0x49
	KeyJ                Key = 0x4a
	KeyK                Key = 0x4b
	KeyL                Key = 0x4c
	KeyM                Key = 0x4d
	KeyN                Key = 0x4e
	KeyO                Key = 0x4f
	KeyP                Key = 0x50
	KeyQ                Key = 0x51
	KeyR                Key = 0x52
	KeyS                Key = 0x53
	KeyT                Key = 0x54
	KeyU                Key = 0x55
	KeyV                Key = 0x56
	KeyW                Key = 0x57
	KeyX                Key = 0x58
	KeyY                Key = 0x59
	KeyZ                Key = 0x5a
	KeyLWin             Key = 0x5b
	KeyRWin             Key = 0x5c
	KeyApps             Key = 0x5d
	KeySleep            Key = 0x5f
	KeyNumpad0          Key = 0x60
	KeyNumpad1          Key = 0x61
	KeyNumpad2          Key = 0x62
	KeyNumpad3          Key = 0x63
	KeyNumpad4          Key = 0x64
	KeyNumpad5          Key = 0x65
	KeyNumpad6          Key = 0x66
	KeyNumpad7          Key = 0x67
	KeyNumpad8          Key = 0x68
	KeyNumpad9          Key = 0x69
	KeyNumpadMult       Key = 0x6a
	KeyNumpadAdd        Key = 0x6b
	KeySeparator        Key = 0x6c
	KeyNumpadSub        Key = 0x6d
	KeyNumpadDot        Key = 0x6e
	KeyNumpadDiv        Key = 0x6f
	KeyF1               Key = 0x70
	KeyF2               Key = 0x71
	KeyF3               Key = 0x72
	KeyF4               Key = 0x73
	KeyF5               Key = 0x74
	KeyF6               Key = 0x75
	KeyF7               Key = 0x76
	KeyF8               Key = 0x77
	KeyF9               Key = 0x78
	KeyF10              Key = 0x79
	KeyF11              Key = 0x7a
	KeyF12              Key = 0x7b
	KeyF13              Key = 0x7c
	KeyF14              Key = 0x7d
	KeyF15              Key = 0x7e
	KeyF16              Key = 0x7f
	KeyF17              Key = 0x80
	KeyF18              Key = 0x81
	KeyF19              Key = 0x82
	KeyF20              Key = 0x83
	KeyF21              Key = 0x84
	KeyF22              Key = 0x85
	KeyF23              Key = 0x86
	KeyF24              Key = 0x87
	KeyNumLock          Key = 0x90
	KeyScrollLock       Key = 0x91
	KeyLShift           Key = 0xa0
	KeyRShift           Key = 0xa1
	KeyLCtrl            Key = 0xa2
	KeyRCtrl            Key = 0xa3
	KeyLAlt             Key = 0xa4
	KeyRAlt             Key = 0xa5
	KeyBrowserBack      Key = 0xa6
	KeyBrowserForward   Key = 0xa7
	KeyBrowserRefresh   Key = 0xa8
	KeyBrowserStop      Key = 0xa9
	KeyBrowserSearch    Key = 0xaa
	KeyBrowserFavorites Key = 0xab
	KeyBrowserHome      Key = 0xac
	KeyVolumeMute       Key = 0xad
	KeyVolumeDown       Key = 0xae
	KeyVolumeUp         Key = 0xaf
	KeyMediaNext        Key = 0xb0
	KeyMediaPrev        Key = 0xb1
	KeyMediaStop        Key = 0xb2
	KeyMediaPlayPause   Key = 0xb3
	KeyLaunchMail       Key = 0xb4
	KeyLaunchMedia      Key = 0xb5
	KeyLaunchApp1       Key = 0xb6
	KeyLaunchApp2       Key = 0xb7
	KeySemicolon        Key = 0xba
	KeyEqual            Key = 0xbb
	KeyComma            Key = 0xbc
	KeyMinus            Key = 0xbd
	KeyPeriod           Key = 0xbe
	KeySlash            Key = 0xbf
	KeyBacktick         Key = 0xc0
	KeyLBracket         Key = 0xdb
	KeyBackslash        Key = 0xdc
	KeyRBracket         Key = 0xdd
	KeyQuote            Key = 0xde
	KeyOEM8             Key = 0xdf
	KeyOEM102           Key = 0xe2
	KeyProcess          Key = 0xe5
	KeyPacket           Key = 0xe7
	KeyAttn             Key = 0xf6
	KeyCrSel            Key = 0xf7
	KeyExSel            Key = 0xf8
	KeyEraseEOF         Key = 0xf9
	KeyPlay             Key = 0xfa
	KeyZoom             Key = 0xfb
	KeyNoName           Key = 0xfc
	KeyPA1              Key = 0xfd
	KeyOEMClear         Key = 0xfe
)

const (
	KeyMouseLeft   = KeyLButton
	KeyMouseRight  = KeyRButton
	KeyMouseMiddle = KeyMButton
	KeyMouseX1     = KeyXButton1
	KeyMouseX2     = KeyXButton2
	KeyReturn      = KeyEnter
	KeyControl     = KeyCtrl
	KeyMenu        = KeyAlt
	KeyEscape      = KeyEsc
	KeyPageUp      = KeyPgUp
	KeyPageDown    = KeyPgDn
	KeyDel         = KeyDelete
	KeyAppsKey     = KeyApps
	KeyBrowserNext = KeyBrowserForward
)

// MouseButton names logical buttons. Primary and secondary respect the user's
// swapped-button setting; KeyLButton and KeyRButton always name physical sides.
type MouseButton uint8

const (
	MousePrimary MouseButton = iota + 1
	MouseSecondary
	MouseMiddle
	MouseX1
	MouseX2
)

// InputSnapshot is an immutable value containing states for the keys supplied
// to PollInput. Its zero value contains no sampled keys.
type InputSnapshot struct {
	sampled [4]uint64
	down    [4]uint64
}

func inputBit(key Key) (int, uint64) {
	return int(key) >> 6, uint64(1) << (uint(key) & 63)
}

// Sampled reports whether key was included in this snapshot.
func (s InputSnapshot) Sampled(key Key) bool {
	if !key.Valid() {
		return false
	}
	word, bit := inputBit(key)
	return s.sampled[word]&bit != 0
}

// Down reports whether a sampled key was down. It returns false for a key that
// was not sampled; use Sampled when that distinction matters.
func (s InputSnapshot) Down(key Key) bool {
	if !key.Valid() {
		return false
	}
	word, bit := inputBit(key)
	return s.sampled[word]&bit != 0 && s.down[word]&bit != 0
}

// PressedSince reports an up-to-down edge between two snapshots. Both
// snapshots must contain key, which prevents an uninitialized baseline from
// being reported as a press.
func (s InputSnapshot) PressedSince(previous InputSnapshot, key Key) bool {
	return s.Sampled(key) && previous.Sampled(key) && s.Down(key) && !previous.Down(key)
}

// ReleasedSince reports a down-to-up edge between two snapshots.
func (s InputSnapshot) ReleasedSince(previous InputSnapshot, key Key) bool {
	return s.Sampled(key) && previous.Sampled(key) && !s.Down(key) && previous.Down(key)
}

// AnyDown reports whether at least one supplied key is sampled and down.
func (s InputSnapshot) AnyDown(keys ...Key) bool {
	for _, key := range keys {
		if s.Down(key) {
			return true
		}
	}
	return false
}

// AllDown reports whether every supplied key is sampled and down. It returns
// false for an empty list so an empty binding cannot trigger accidentally.
func (s InputSnapshot) AllDown(keys ...Key) bool {
	if len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		if !s.Down(key) {
			return false
		}
	}
	return true
}

func makeInputSnapshot(keys []Key, read func(Key) bool) (InputSnapshot, error) {
	if len(keys) == 0 || read == nil {
		return InputSnapshot{}, ErrInvalidArgument
	}
	for _, key := range keys {
		if !key.Valid() {
			return InputSnapshot{}, fmt.Errorf("%w: invalid virtual key 0x%02x", ErrInvalidArgument, uint8(key))
		}
	}
	var snapshot InputSnapshot
	for _, key := range keys {
		word, bit := inputBit(key)
		if snapshot.sampled[word]&bit != 0 {
			continue
		}
		snapshot.sampled[word] |= bit
		if read(key) {
			snapshot.down[word] |= bit
		}
	}
	return snapshot, nil
}

// ParseKey parses common AutoHotkey/Win32 names, single ASCII letters and
// digits, F1 through F24, Numpad0 through Numpad9, or VK_XX hexadecimal codes.
func ParseKey(name string) (Key, error) {
	raw := strings.TrimSpace(name)
	if raw == "-" {
		return KeyMinus, nil
	}
	normalized := strings.ToUpper(keyNameReplacer.Replace(raw))
	if len(normalized) == 1 {
		c := normalized[0]
		if c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' {
			return Key(c), nil
		}
	}
	if strings.HasPrefix(normalized, "F") {
		if n, err := strconv.Atoi(normalized[1:]); err == nil && n >= 1 && n <= 24 {
			return Key(int(KeyF1) + n - 1), nil
		}
	}
	if strings.HasPrefix(normalized, "NUMPAD") && len(normalized) == len("NUMPAD0") {
		c := normalized[len(normalized)-1]
		if c >= '0' && c <= '9' {
			return Key(int(KeyNumpad0) + int(c-'0')), nil
		}
	}
	if strings.HasPrefix(normalized, "VK") {
		hex := strings.TrimPrefix(normalized[2:], "0X")
		if code, err := strconv.ParseUint(hex, 16, 8); err == nil {
			key := Key(code)
			if key.Valid() {
				return key, nil
			}
		}
	}
	if key, ok := namedKeys[normalized]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("%w: unknown key %q", ErrInvalidArgument, name)
}

var keyNameReplacer = strings.NewReplacer("_", "", "-", "", " ", "")

func (k Key) String() string {
	if k >= Key0 && k <= Key9 || k >= KeyA && k <= KeyZ {
		return string(rune(k))
	}
	if k >= KeyF1 && k <= KeyF24 {
		return fmt.Sprintf("F%d", int(k-KeyF1)+1)
	}
	if k >= KeyNumpad0 && k <= KeyNumpad9 {
		return fmt.Sprintf("Numpad%d", int(k-KeyNumpad0))
	}
	if name, ok := keyNames[k]; ok {
		return name
	}
	return fmt.Sprintf("VK_%02X", uint8(k))
}

var keyNames = map[Key]string{
	KeyLButton: "LButton", KeyRButton: "RButton", KeyCancel: "Cancel",
	KeyMButton: "MButton", KeyXButton1: "XButton1", KeyXButton2: "XButton2",
	KeyBackspace: "Backspace", KeyTab: "Tab", KeyClear: "Clear", KeyEnter: "Enter",
	KeyShift: "Shift", KeyCtrl: "Ctrl", KeyAlt: "Alt", KeyPause: "Pause",
	KeyCapsLock: "CapsLock", KeyKana: "Kana", KeyIMEOn: "IMEOn", KeyJunja: "Junja",
	KeyFinal: "Final", KeyHanja: "Hanja", KeyIMEOff: "IMEOff", KeyEsc: "Esc",
	KeyConvert: "Convert", KeyNonConvert: "NonConvert", KeyAccept: "Accept",
	KeyModeChange: "ModeChange", KeySpace: "Space", KeyPgUp: "PgUp", KeyPgDn: "PgDn",
	KeyEnd: "End", KeyHome: "Home", KeyLeft: "Left", KeyUp: "Up", KeyRight: "Right",
	KeyDown: "Down", KeySelect: "Select", KeyPrint: "Print", KeyExecute: "Execute",
	KeyPrintScreen: "PrintScreen", KeyInsert: "Insert", KeyDelete: "Delete", KeyHelp: "Help",
	KeyLWin: "LWin", KeyRWin: "RWin", KeyApps: "Apps", KeySleep: "Sleep",
	KeyNumpadMult: "NumpadMult", KeyNumpadAdd: "NumpadAdd", KeySeparator: "Separator",
	KeyNumpadSub: "NumpadSub", KeyNumpadDot: "NumpadDot", KeyNumpadDiv: "NumpadDiv",
	KeyNumLock: "NumLock", KeyScrollLock: "ScrollLock", KeyLShift: "LShift",
	KeyRShift: "RShift", KeyLCtrl: "LCtrl", KeyRCtrl: "RCtrl", KeyLAlt: "LAlt",
	KeyRAlt: "RAlt", KeyBrowserBack: "BrowserBack", KeyBrowserForward: "BrowserForward",
	KeyBrowserRefresh: "BrowserRefresh", KeyBrowserStop: "BrowserStop",
	KeyBrowserSearch: "BrowserSearch", KeyBrowserFavorites: "BrowserFavorites",
	KeyBrowserHome: "BrowserHome", KeyVolumeMute: "VolumeMute", KeyVolumeDown: "VolumeDown",
	KeyVolumeUp: "VolumeUp", KeyMediaNext: "MediaNext", KeyMediaPrev: "MediaPrev",
	KeyMediaStop: "MediaStop", KeyMediaPlayPause: "MediaPlayPause", KeyLaunchMail: "LaunchMail",
	KeyLaunchMedia: "LaunchMedia", KeyLaunchApp1: "LaunchApp1", KeyLaunchApp2: "LaunchApp2",
	KeySemicolon: "Semicolon", KeyEqual: "Equal", KeyComma: "Comma", KeyMinus: "Minus",
	KeyPeriod: "Period", KeySlash: "Slash", KeyBacktick: "Backtick", KeyLBracket: "LBracket",
	KeyBackslash: "Backslash", KeyRBracket: "RBracket", KeyQuote: "Quote", KeyOEM8: "OEM8",
	KeyOEM102: "OEM102", KeyProcess: "Process", KeyPacket: "Packet", KeyAttn: "Attn",
	KeyCrSel: "CrSel", KeyExSel: "ExSel", KeyEraseEOF: "EraseEOF", KeyPlay: "Play",
	KeyZoom: "Zoom", KeyNoName: "NoName", KeyPA1: "PA1", KeyOEMClear: "OEMClear",
}

var namedKeys = func() map[string]Key {
	keys := make(map[string]Key, len(keyNames)+50)
	for key, name := range keyNames {
		keys[strings.ToUpper(name)] = key
	}
	for name, key := range map[string]Key{
		"BS": KeyBackspace, "RETURN": KeyEnter, "CONTROL": KeyCtrl, "MENU": KeyAlt,
		"ESCAPE": KeyEsc, "PAGEUP": KeyPgUp, "PAGEDOWN": KeyPgDn, "PRIOR": KeyPgUp,
		"NEXT": KeyPgDn, "SNAPSHOT": KeyPrintScreen, "INS": KeyInsert, "DEL": KeyDelete,
		"CTRLBREAK": KeyCancel, "BREAK": KeyPause, "APPSKEY": KeyApps,
		"MOUSELEFT": KeyLButton, "MOUSERIGHT": KeyRButton, "MOUSEMIDDLE": KeyMButton,
		"MOUSEX1": KeyXButton1, "MOUSEX2": KeyXButton2,
		"BROWSERNEXT":    KeyBrowserForward,
		"MEDIANEXTTRACK": KeyMediaNext, "MEDIAPREVTRACK": KeyMediaPrev,
		"MEDIAPREVIOUSTRACK": KeyMediaPrev, "MEDIAPLAYPAUSE": KeyMediaPlayPause,
		"LAUNCHMEDIASELECT": KeyLaunchMedia,
		"NUMPADINS":         KeyInsert, "NUMPADEND": KeyEnd, "NUMPADDOWN": KeyDown,
		"NUMPADPGDN": KeyPgDn, "NUMPADLEFT": KeyLeft, "NUMPADCLEAR": KeyClear,
		"NUMPADRIGHT": KeyRight, "NUMPADHOME": KeyHome, "NUMPADUP": KeyUp,
		"NUMPADPGUP": KeyPgUp, "NUMPADDEL": KeyDelete,
		";": KeySemicolon, "=": KeyEqual, ",": KeyComma, "-": KeyMinus,
		".": KeyPeriod, "/": KeySlash, "`": KeyBacktick, "[": KeyLBracket,
		"\\": KeyBackslash, "]": KeyRBracket, "'": KeyQuote,
	} {
		keys[name] = key
	}
	return keys
}()
