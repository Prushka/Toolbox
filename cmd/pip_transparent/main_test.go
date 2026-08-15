package main

import "testing"

func TestIsPiPTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{name: "Chrome", title: "Picture in picture", want: true},
		{name: "Firefox", title: "Picture-in-Picture", want: true},
		{name: "case insensitive", title: "PICTURE IN PICTURE", want: true},
		{name: "PiP-related page title", title: "How to use Picture-in-Picture", want: false},
		{name: "normal browser window", title: "Video - YouTube", want: false},
		{name: "letters embedded in word", title: "Pipette", want: false},
		{name: "empty", title: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPiPTitle(tt.title); got != tt.want {
				t.Fatalf("isPiPTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}
