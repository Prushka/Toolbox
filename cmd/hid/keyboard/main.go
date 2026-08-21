// Command keyboard demonstrates chords, held modifiers, text, and special keys
// by opening Notepad and typing into a new document.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/Prushka/Toolbox/cmd/hid/internal/example"
	"github.com/Prushka/Toolbox/hid"
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
	device, port, err := example.Open(ctx, *portName, hid.WithTapDelay(*tapDelay))
	if err != nil {
		log.Fatal(err)
	}
	defer example.Close(device)

	err = device.Do(ctx,
		hid.Pause(*startDelay),
		hid.TapKeys(hid.GUI, hid.MustKey('r')),
		hid.Pause(200*time.Millisecond),
		hid.WriteText("notepad"),
		hid.TapKeys(hid.KeyEnter),
		hid.Pause(750*time.Millisecond),
		hid.HoldKeys(hid.Shift),
		hid.TapKeys(hid.MustKey('t')),
		hid.ReleaseKeys(hid.Shift),
		hid.WriteText("oolbox keyboard example\n"),
		hid.WriteText(*text),
		hid.TapKeys(hid.KeyEnter),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("keyboard example completed on %s\n", port.Name)
}
