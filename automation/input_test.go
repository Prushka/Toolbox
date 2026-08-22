package automation

import (
	"errors"
	"testing"
)

func TestParseKey(t *testing.T) {
	tests := map[string]Key{
		"a":                KeyA,
		"9":                Key9,
		"F1":               KeyF1,
		"f24":              KeyF24,
		"Numpad7":          KeyNumpad7,
		"Page Down":        KeyPgDn,
		"page-down":        KeyPgDn,
		"LButton":          KeyLButton,
		"mouse_right":      KeyRButton,
		"Control":          KeyCtrl,
		"VK_1B":            KeyEsc,
		"vk0xba":           KeySemicolon,
		"BrowserRefresh":   KeyBrowserRefresh,
		"Browser_Forward":  KeyBrowserForward,
		"AppsKey":          KeyApps,
		"Media_Play_Pause": KeyMediaPlayPause,
		"NumpadPgDn":       KeyPgDn,
		";":                KeySemicolon,
		"-":                KeyMinus,
	}
	for input, want := range tests {
		got, err := ParseKey(input)
		if err != nil || got != want {
			t.Errorf("ParseKey(%q) = %v, %v; want %v", input, got, err, want)
		}
	}
	for _, input := range []string{"", "F0", "F25", "Numpad10", "VK_00", "VK_FF", "WheelUp", "not-a-key"} {
		if _, err := ParseKey(input); !errors.Is(err, ErrInvalidArgument) {
			t.Errorf("ParseKey(%q) error = %v; want ErrInvalidArgument", input, err)
		}
	}
}

func TestKeyStringRoundTrip(t *testing.T) {
	for key, name := range keyNames {
		got, err := ParseKey(name)
		if err != nil || got != key {
			t.Errorf("ParseKey(%q) = %v, %v; want %v", name, got, err, key)
		}
		if got := key.String(); got != name {
			t.Errorf("Key(%02X).String() = %q; want %q", uint8(key), got, name)
		}
	}
	for _, key := range []Key{
		KeyLButton, KeyEnter, KeyPgDn, KeyA, Key0, KeyNumpad4, KeyF24,
		KeyLCtrl, KeyVolumeUp, KeySemicolon, KeyOEM102, Key(0x88),
	} {
		text := key.String()
		got, err := ParseKey(text)
		if err != nil || got != key {
			t.Errorf("ParseKey(%q) = %v, %v; want %v", text, got, err, key)
		}
	}
	if got := Key(0).String(); got != "VK_00" {
		t.Fatalf("invalid key String() = %q", got)
	}
}

func TestInputSnapshotStateAndEdges(t *testing.T) {
	reads := map[Key]int{}
	previous, err := makeInputSnapshot([]Key{KeyCtrl, KeyF8, KeyCtrl}, func(key Key) bool {
		reads[key]++
		return key == KeyCtrl
	})
	if err != nil {
		t.Fatal(err)
	}
	if reads[KeyCtrl] != 1 || reads[KeyF8] != 1 {
		t.Fatalf("duplicate reads = %#v", reads)
	}
	if !previous.Sampled(KeyCtrl) || !previous.Down(KeyCtrl) || previous.Down(KeyF8) || previous.Sampled(KeyA) {
		t.Fatalf("unexpected previous state: %+v", previous)
	}
	if !previous.AllDown(KeyCtrl) || previous.AllDown(KeyCtrl, KeyF8) || previous.AllDown() {
		t.Fatal("AllDown result is incorrect")
	}
	if !previous.AnyDown(KeyF8, KeyCtrl) || previous.AnyDown(KeyF8, KeyA) {
		t.Fatal("AnyDown result is incorrect")
	}

	current, err := makeInputSnapshot([]Key{KeyCtrl, KeyF8}, func(Key) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if !current.PressedSince(previous, KeyF8) || current.PressedSince(previous, KeyCtrl) {
		t.Fatal("pressed edge is incorrect")
	}
	if current.PressedSince(InputSnapshot{}, KeyF8) {
		t.Fatal("uninitialized baseline produced a pressed edge")
	}

	released, err := makeInputSnapshot([]Key{KeyCtrl, KeyF8}, func(Key) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	if !released.ReleasedSince(current, KeyCtrl) || !released.ReleasedSince(current, KeyF8) {
		t.Fatal("released edge is incorrect")
	}
	if current.ReleasedSince(previous, KeyA) {
		t.Fatal("unsampled key produced a released edge")
	}
}

func TestInputSnapshotRejectsInvalidKeys(t *testing.T) {
	called := false
	read := func(Key) bool { called = true; return false }
	if _, err := makeInputSnapshot(nil, read); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("empty keys error = %v", err)
	}
	if _, err := makeInputSnapshot([]Key{KeyA}, nil); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("nil reader error = %v", err)
	}
	if _, err := makeInputSnapshot([]Key{KeyA, 0}, read); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid key error = %v", err)
	}
	if called {
		t.Fatal("reader called before validation completed")
	}
}
