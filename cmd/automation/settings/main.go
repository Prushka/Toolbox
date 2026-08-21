package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Prushka/Toolbox/automation"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "settings:", err)
		os.Exit(1)
	}
}

func run() (err error) {
	iniPath := flag.String("ini", "automation-example.ini", "INI file path")
	section := flag.String("section", "automation", "INI section")
	key := flag.String("key", "enabled", "INI key")
	defaultValue := flag.String("default", "false", "value returned when the key is absent")
	write := flag.Bool("write", false, "write -value before reading")
	value := flag.String("value", "true", "value written with -write")
	logPath := flag.String("log", "automation-example.log", "append log path")
	flag.Parse()

	if *write {
		if err := automation.WriteINI(*iniPath, *section, *key, *value); err != nil {
			return err
		}
	}
	got, err := automation.ReadINI(*iniPath, *section, *key, *defaultValue)
	if err != nil {
		return err
	}
	logger, err := automation.OpenLogger(*logPath)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := logger.Close(); err == nil {
			err = closeErr
		}
	}()
	logger.SetPrefix("settings")
	if err := logger.PrintfErr("%s [%s] %s=%s", *iniPath, *section, *key, got); err != nil {
		return err
	}
	fmt.Printf("[%s] %s=%s (logged to %s)\n", *section, *key, got, *logPath)
	return nil
}
