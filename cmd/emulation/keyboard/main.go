// Command keyboard demonstrates chords, held modifiers, text, and special keys
// by opening Notepad and typing into a new document.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Prushka/Toolbox/cmd/emulation/internal/example"
	"github.com/Prushka/Toolbox/emulation"
)

func main() {
	portName := flag.String("port", "", "Arduino Leonardo CDC port (auto-detected when empty)")
	run := flag.Bool("run", false, "send the keyboard demonstration")
	text := flag.String("text", "Typed through the Arduino Leonardo.", "US-ASCII text to type")
	startDelay := flag.Duration("start-delay", time.Second, "delay before sending input")
	tapDelay := flag.Duration("tap-delay", 20*time.Millisecond, "key hold time")
	flag.Parse()
	if !*run {
		log.Fatal("keyboard input is disabled; pass -run to execute the example")
	}
	if *startDelay < 0 {
		log.Fatal("-start-delay cannot be negative")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, port, err := example.Open(ctx, *portName, emulation.WithTapDelay(*tapDelay))
	if err != nil {
		log.Fatal(err)
	}
	defer example.Close(device)

	err = device.Do(ctx,
		emulation.Pause(*startDelay),
		emulation.TapKeys(emulation.GUI, emulation.MustKey('r')),
		emulation.Pause(200*time.Millisecond),
		emulation.WriteText("notepad"),
		emulation.TapKeys(emulation.KeyEnter),
		emulation.Pause(750*time.Millisecond),
		emulation.HoldKeys(emulation.Shift),
		emulation.TapKeys(emulation.MustKey('t')),
		emulation.ReleaseKeys(emulation.Shift),
		emulation.WriteText("oolbox keyboard example\n"),
		emulation.WriteText(*text),
		emulation.TapKeys(emulation.KeyEnter),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("keyboard example completed on %s\n", port.Name)
}
