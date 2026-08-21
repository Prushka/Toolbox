// Command actions demonstrates the composable Action API, including chords,
// held keys, pauses, repeated groups, text, and reversible pointer movement.
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
	run := flag.Bool("run", false, "send the composed action workflow")
	repeat := flag.Int("repeat", 3, "number of repeated text lines")
	flag.Parse()
	if !*run {
		log.Fatal("input is disabled; pass -run to execute the example")
	}
	if *repeat < 0 {
		log.Fatal("-repeat cannot be negative")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, port, err := example.Open(ctx, *portName)
	if err != nil {
		log.Fatal(err)
	}
	defer example.Close(device)

	err = device.Do(ctx,
		hid.TapKeys(hid.GUI, hid.MustKey('r')),
		hid.Pause(200*time.Millisecond),
		hid.WriteText("notepad"),
		hid.TapKeys(hid.KeyEnter),
		hid.Pause(750*time.Millisecond),
		hid.WriteText("Toolbox composable actions\n"),
		hid.Repeat(*repeat,
			hid.WriteText("Repeated action line"),
			hid.TapKeys(hid.KeyEnter),
		),
		hid.HoldKeys(hid.Shift),
		hid.TapKeys(hid.MustKey('1')),
		hid.ReleaseKeys(hid.Shift),
		hid.MoveBy(20, 0),
		hid.Pause(200*time.Millisecond),
		hid.MoveBy(-20, 0),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("action workflow completed on %s\n", port.Name)
}
