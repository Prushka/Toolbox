package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Prushka/Toolbox/automation"
	"github.com/Prushka/Toolbox/cmd/automation/internal/example"
	"github.com/rs/zerolog/log"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("image-search failed")
	}
}

func run() error {
	var target example.QueryFlags
	target.Bind(flag.CommandLine, true)
	imagePath := flag.String("image", "", "template image path")
	spec := flag.String("spec", "", "AHK-style options and image path")
	variation := flag.Uint("variation", 0, "per-channel variation from 0 to 255")
	transparentText := flag.String("transparent", "", "wildcard color as RRGGBB")
	width := flag.Int("width", 0, "scaled template width; -1 preserves aspect ratio")
	height := flag.Int("height", 0, "scaled template height; -1 preserves aspect ratio")
	regionText := flag.String("region", "", "optional client rectangle as left,top,right,bottom")
	timeout := flag.Duration("timeout", 0, "poll until found or this timeout; zero searches once")
	interval := flag.Duration("interval", 50*time.Millisecond, "polling interval")
	flag.Parse()

	options, path, err := imageOptions(*spec, *imagePath, *variation, *transparentText, *width, *height)
	if err != nil {
		return err
	}
	region, err := example.ParseRect(*regionText)
	if err != nil {
		return err
	}
	template, err := automation.LoadTemplate(path, options)
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

	var point automation.Point
	search := func() (bool, error) {
		var found bool
		var err error
		point, found, err = window.SearchTemplate(region, template)
		return found, err
	}
	if *timeout <= 0 {
		found, err := search()
		if err != nil {
			return err
		}
		if !found {
			fmt.Println("image not found")
			return nil
		}
	} else {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		ctx, cancel := context.WithTimeout(ctx, *timeout)
		defer cancel()
		if err := automation.WaitUntil(ctx, *interval, search); err != nil {
			return err
		}
	}

	fmt.Printf("found %s at client coordinate %d,%d\n", path, point.X, point.Y)
	return nil
}

func imageOptions(spec, path string, variation uint, transparent string, width, height int) (automation.ImageSearchOptions, string, error) {
	if spec != "" {
		if path != "" || variation != 0 || transparent != "" || width != 0 || height != 0 {
			return automation.ImageSearchOptions{}, "", fmt.Errorf("-spec cannot be combined with individual image options")
		}
		options, parsedPath, err := automation.ParseImageSearchOptions(spec)
		if err != nil {
			return options, "", err
		}
		if parsedPath == "" {
			return options, "", fmt.Errorf("-spec must include an image path")
		}
		return options, parsedPath, nil
	}
	if path == "" {
		return automation.ImageSearchOptions{}, "", fmt.Errorf("-image or -spec is required")
	}
	if variation > 255 {
		return automation.ImageSearchOptions{}, "", fmt.Errorf("variation must be from 0 to 255")
	}
	options := automation.ImageSearchOptions{Variation: uint8(variation), Width: width, Height: height}
	if transparent != "" {
		color, err := example.ParseRGB(transparent)
		if err != nil {
			return options, "", err
		}
		options.Transparent = &color
	}
	return options, path, nil
}
