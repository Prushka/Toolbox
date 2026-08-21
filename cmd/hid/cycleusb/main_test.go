package main

import (
	"testing"

	"github.com/Prushka/Toolbox/hid"
)

func TestFindTargetBySerialAfterPortChange(t *testing.T) {
	target := hid.Port{Name: "COM5", SerialNumber: "board-1"}
	ports := []hid.Port{{Name: "COM7", SerialNumber: "board-1"}}
	got, ok := findTarget(target, ports)
	if !ok || got.Name != "COM7" {
		t.Fatalf("findTarget = (%#v, %v), want COM7", got, ok)
	}
}

func TestFindTargetByNameWithoutSerial(t *testing.T) {
	target := hid.Port{Name: "COM5"}
	ports := []hid.Port{{Name: "com5"}, {Name: "COM7"}}
	got, ok := findTarget(target, ports)
	if !ok || got.Name != "com5" {
		t.Fatalf("findTarget = (%#v, %v), want com5", got, ok)
	}
}
