package main

import (
	"reflect"
	"testing"

	"github.com/Prushka/Toolbox/hid"
)

func TestParseChord(t *testing.T) {
	t.Parallel()
	got, err := parseChord("ALT, TAB")
	if err != nil {
		t.Fatal(err)
	}
	want := []hid.Key{hid.Alt, hid.KeyTab}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseChord = %v, want %v", got, want)
	}
}

func TestParseChordRejectsEmptyKey(t *testing.T) {
	t.Parallel()
	if _, err := parseChord("ALT,"); err == nil {
		t.Fatal("parseChord accepted an empty key")
	}
}
