package example

import (
	"strings"
	"testing"

	"github.com/Prushka/Toolbox/emulation"
)

func TestSelectPort(t *testing.T) {
	want := emulation.Port{Name: "COM5"}
	got, err := SelectPort([]emulation.Port{want})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name {
		t.Fatalf("SelectPort name = %q, want %q", got.Name, want.Name)
	}
}

func TestSelectPortRequiresExactlyOne(t *testing.T) {
	for _, ports := range [][]emulation.Port{
		nil,
		{{Name: "COM5"}, {Name: "COM7"}},
	} {
		_, err := SelectPort(ports)
		if err == nil || !strings.Contains(err.Error(), "pass -port explicitly") {
			t.Fatalf("SelectPort(%v) error = %v", ports, err)
		}
	}
}
