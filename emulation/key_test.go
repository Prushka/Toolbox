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
