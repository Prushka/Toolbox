// Command actions demonstrates the composable Action API, including chords,
// held keys, pauses, repeated groups, text, and reversible pointer movement.
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
		emulation.TapKeys(emulation.GUI, emulation.MustKey('r')),
		emulation.Pause(200*time.Millisecond),
		emulation.WriteText("notepad"),
		emulation.TapKeys(emulation.KeyEnter),
		emulation.Pause(750*time.Millisecond),
		emulation.WriteText("Toolbox composable actions\n"),
		emulation.Repeat(*repeat,
			emulation.WriteText("Repeated action line"),
			emulation.TapKeys(emulation.KeyEnter),
		),
		emulation.HoldKeys(emulation.Shift),
		emulation.TapKeys(emulation.MustKey('1')),
		emulation.ReleaseKeys(emulation.Shift),
		emulation.MoveBy(20, 0),
		emulation.Pause(200*time.Millisecond),
		emulation.MoveBy(-20, 0),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("action workflow completed on %s\n", port.Name)
}
