package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Prushka/Toolbox/automation"
	"github.com/Prushka/Toolbox/cmd/automation/internal/example"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "windows:", err)
		os.Exit(1)
	}
}

func run() error {
	var filters example.QueryFlags
	filters.Bind(flag.CommandLine, true)
	limit := flag.Int("limit", 25, "maximum windows to print; zero prints all")
	showProcessPath := flag.Bool("process-path", false, "query and print each process path")
	flag.Parse()

	if *limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if err := automation.SetDPIAware(); err != nil && !errors.Is(err, automation.ErrUnsupported) {
		return err
	}
	query, err := filters.Query()
	if err != nil {
		return err
	}
	windows, err := automation.FindWindows(query)
	if err != nil {
		return err
	}
	if len(windows) == 0 {
		fmt.Println("no matching windows")
		return nil
	}

	printed := len(windows)
	if *limit > 0 && printed > *limit {
		printed = *limit
	}
	for _, window := range windows[:printed] {
		rect, rectErr := window.Rect()
		rectText := "unavailable"
		if rectErr == nil {
			rectText = fmt.Sprintf("%d,%d,%d,%d", rect.Left, rect.Top, rect.Right, rect.Bottom)
		}
		fmt.Printf("hwnd=0x%X pid=%d dpi=%d visible=%t minimized=%t maximized=%t rect=%s title=%q class=%q\n",
			window.Handle(), window.PID(), automation.WindowDPI(window), window.IsVisible(),
			window.IsMinimized(), window.IsMaximized(), rectText, window.Title(), window.Class())
		if *showProcessPath {
			fmt.Printf("  process=%q\n", window.ProcessPath())
		}
	}
	if printed < len(windows) {
		fmt.Printf("... %d more matches (increase -limit)\n", len(windows)-printed)
	}
	return nil
}
