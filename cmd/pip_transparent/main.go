package main

import (
	"fmt"
	"os"
	"strings"
	"unicode"
)

type toggleResult struct {
	handle       uintptr
	title        string
	clickThrough bool
}

func main() {
	results, err := togglePiPWindows()
	for _, result := range results {
		state := "clickable"
		if result.clickThrough {
			state = "click-through"
		}
		fmt.Printf("0x%X is now %s: %q\n", result.handle, state, result.title)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "pip-toggle:", err)
		os.Exit(1)
	}
}

func isPiPTitle(title string) bool {
	var normalized strings.Builder
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			normalized.WriteRune(unicode.ToLower(r))
		}
	}

	return normalized.String() == "pictureinpicture"
}
