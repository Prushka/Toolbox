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
		log.Fatal().Err(err).Msg("app-toggle failed")
	}
}

func run() error {
	var target example.QueryFlags
	target.Bind(flag.CommandLine, false)
	command := flag.String("command", "", "command to start when the window is absent")
	var args example.Strings
	flag.Var(&args, "arg", "one command argument; repeat for multiple arguments")
	flag.Parse()

	if !target.HasSelector() {
		return fmt.Errorf("at least one window selector is required")
	}
	query, err := target.Query()
	if err != nil {
		return err
	}
	if err := automation.SetDPIAware(); err != nil && !errors.Is(err, automation.ErrUnsupported) {
		return err
	}
	app := automation.App{Query: query, Command: *command, Args: args}
	window, process, err := app.ToggleOrStart()
	if err != nil {
		return err
	}
	if process != nil {
		fmt.Printf("started pid=%d command=%q\n", process.Process.Pid, process.Path)
		return nil
	}
	fmt.Printf("toggled hwnd=0x%X title=%q\n", window.Handle(), window.Title())
	return nil
}
