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
		fmt.Fprintln(os.Stderr, "pixel-search:", err)
		os.Exit(1)
	}
}

func run() error {
	var target example.QueryFlags
	target.Bind(flag.CommandLine, true)
	colorText := flag.String("color", "", "required target color as RRGGBB")
	toleranceText := flag.String("tolerance", "0", "channel tolerance as n or r,g,b")
	regionText := flag.String("region", "", "optional client rectangle as left,top,right,bottom")
	flag.Parse()

	if *colorText == "" {
		return fmt.Errorf("-color is required")
	}
	want, err := example.ParseRGB(*colorText)
	if err != nil {
		return err
	}
	tolerance, err := example.ParseTolerance(*toleranceText)
	if err != nil {
		return err
	}
	region, err := example.ParseRect(*regionText)
	if err != nil {
		return err
	}
	if err := automation.SetDPIAware(); err != nil && !errors.Is(err, automation.ErrUnsupported) {
		return err
	}
	window, err := target.Resolve()
	if err != nil {
		return err
	}

	point, found, err := window.SearchPixel(region, want, tolerance)
	if err != nil {
		return err
	}
	if !found {
		fmt.Println("color not found")
		return nil
	}
	fmt.Printf("found #%06X at client coordinate %d,%d\n", want.Uint32(), point.X, point.Y)
	return nil
}
