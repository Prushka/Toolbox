package main

import (
	"testing"

	"github.com/Prushka/Toolbox/automation"
)

func TestImageOptionsIndividualFlags(t *testing.T) {
	options, path, err := imageOptions("", "target.png", 17, "FF00AA", 100, -1)
	if err != nil {
		t.Fatal(err)
	}
	if path != "target.png" || options.Variation != 17 || options.Width != 100 || options.Height != -1 {
		t.Fatalf("options=%+v path=%q", options, path)
	}
	if options.Transparent == nil || *options.Transparent != (automation.RGB{R: 255, G: 0, B: 170}) {
		t.Fatalf("transparent=%v", options.Transparent)
	}
}

func TestImageOptionsSpecAndConflicts(t *testing.T) {
	options, path, err := imageOptions(`*42 *TransBlack *w100 *h-1 "target image.png"`, "", 0, "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if path != "target image.png" || options.Variation != 42 || options.Width != 100 || options.Height != -1 {
		t.Fatalf("options=%+v path=%q", options, path)
	}
	if _, _, err := imageOptions("*42 target.png", "other.png", 0, "", 0, 0); err == nil {
		t.Fatal("expected conflicting options error")
	}
	if _, _, err := imageOptions("", "target.png", 256, "", 0, 0); err == nil {
		t.Fatal("expected variation range error")
	}
}
