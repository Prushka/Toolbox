package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Prushka/Toolbox/automation"
	"github.com/Prushka/Toolbox/cmd/automation/internal/example"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("capture failed")
	}
}

func run() error {
	var target example.QueryFlags
	target.Bind(flag.CommandLine, true)
	screenSpec := flag.String("screen", "", "screen rectangle as left,top,right,bottom")
	regionSpec := flag.String("region", "", "client rectangle as left,top,right,bottom")
	output := flag.String("output", "automation-capture.png", "output PNG path")
	methodName := flag.String("method", "visible", "capture method: visible, print, or auto")
	clientOnly := flag.Bool("client", true, "capture only the client area when no region is set")
	flag.Parse()

	method, err := example.ParseCaptureMethod(*methodName)
	if err != nil {
		return err
	}
	if err := automation.SetDPIAware(); err != nil && !errors.Is(err, automation.ErrUnsupported) {
		return err
	}
	if *regionSpec != "" && !*clientOnly {
		return fmt.Errorf("-region is always client-relative; omit -client=false")
	}

	var bitmap *automation.Bitmap
	if *screenSpec != "" {
		if *regionSpec != "" || target.HasSelector() {
			return fmt.Errorf("-screen cannot be combined with a window selector or -region")
		}
		if method != automation.CaptureVisible {
			return fmt.Errorf("screen capture supports only the visible method")
		}
		rect, err := example.ParseRect(*screenSpec)
		if err != nil {
			return err
		}
		bitmap, err = automation.CaptureScreen(rect)
		if err != nil {
			return err
		}
	} else {
		window, err := target.Resolve()
		if err != nil {
			return err
		}
		options := automation.CaptureOptions{ClientOnly: *clientOnly, Method: method}
		if *regionSpec == "" {
			bitmap, err = window.CaptureWith(options)
		} else {
			region, parseErr := example.ParseRect(*regionSpec)
			if parseErr != nil {
				return parseErr
			}
			options.ClientOnly = true
			bitmap, err = window.CaptureRegion(region, options)
		}
		if err != nil {
			return err
		}
	}

	if err := bitmap.SavePNG(*output); err != nil {
		return err
	}
	fmt.Printf("saved %dx%d capture to %s\n", bitmap.Width, bitmap.Height, *output)
	return nil
}
