package emulation

import "testing"

func TestRuneKey(t *testing.T) {
	tests := []struct {
		rune rune
		want Key
	}{
		{rune: 'a', want: Key('a')},
		{rune: 'Z', want: Key('z')},
		{rune: '/', want: Key('/')},
		{rune: ' ', want: Key(' ')},
	}
	for _, test := range tests {
		got, err := RuneKey(test.rune)
		if err != nil || got != test.want {
			t.Fatalf("RuneKey(%q) = (%v, %v), want (%v, nil)", test.rune, got, err, test.want)
		}
	}
	if _, err := RuneKey('\n'); err == nil {
		t.Fatal("RuneKey accepted a control character")
	}
}

func TestSupportedKeyValues(t *testing.T) {
	for _, key := range []Key{
		MustKey('a'), KeyLeftCtrl, KeyTab, KeyPrintScreen, KeyKeypadEnter,
		KeyMenu, KeyF24,
	} {
		if !isSupportedKey(key) {
			t.Errorf("key 0x%02X is unexpectedly unsupported", byte(key))
		}
	}
	for _, key := range []Key{0, 0x1F, 0x88, 0xB4, 0xC0, 0xEC, 0xEE, 0xFC} {
		if isSupportedKey(key) {
			t.Errorf("key 0x%02X is unexpectedly supported", byte(key))
		}
	}
}
