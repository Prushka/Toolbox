# Leonardo hardware keyboard and mouse

This package controls an Arduino Leonardo over its USB CDC COM port while the
same physical board emits input through Windows' built-in USB HID driver. It
does not install an emulation driver, edit the registry, modify the Windows
driver store, or patch Arduino's installed `boards.txt`.

The firmware is a composite USB device with three functions:

- CDC serial command channel
- USB keyboard
- USB mouse (relative five-button mouse, two-axis wheel, and absolute pointer)

USB VID/PID belongs to the whole physical USB device, not to each interface.
One Leonardo therefore cannot expose a different VID/PID for its keyboard and
mouse interfaces. Both HID interfaces use the real `046D:C223` pair already
named in `references/keyboard.ino`. The Linux USB ID database identifies that
pair as `Logitech, Inc. G11/G15 Keyboard / USB Hub`. That VID belongs to
Logitech; this firmware is not a genuine Logitech device and the identifier is
suitable only for controlled compatibility testing. Windows still
distinguishes the board by its descriptors, CDC interface, physical USB path,
and container. Avoid connecting a real `046D:C223` device at the same time if
software selects devices by VID/PID alone.

This design is not a stealth or anti-detection mechanism. A single Leonardo
cannot accurately reproduce two physical Logitech products, and its composite
descriptor intentionally retains a CDC command channel. Software that inspects
USB topology or descriptors can distinguish it. The library does not include
anti-cheat, endpoint-monitoring, or security-control bypasses.

The CDC port is a local trust boundary, not an authenticated channel. CRC-8
detects accidental transport corruption but does not authenticate commands.
Run command-producing software only on a trusted Windows account and do not
expose the serial stream through a network bridge without adding authentication.

The official Leonardo documentation confirms that its ATmega32u4 can appear as
a keyboard and mouse alongside a virtual CDC serial port:
https://docs.arduino.cc/hardware/leonardo/

The USB ID source is:
https://github.com/usbids/usbids/blob/master/usb.ids

## Firmware safety

Requests are versioned, length-delimited, CRC-8 protected, sequenced, and
acknowledged. Payloads are capped at 64 bytes, abandoned partial frames expire,
and corrupt responses can be resynchronized. `Close`, `CycleUSB`, malformed
multi-key reports, and a 30-second command watchdog release all held keys and
buttons. Cleanup calls have bounded timeouts so cancellation or disconnects do
not hang the caller. Physical unplug is safe; Windows tears down the HID device
and the board starts with an all-released report on reconnect.

Dropping the CDC port's DTR/RTS control lines also releases input immediately;
the watchdog remains a fallback for a host that freezes without closing USB.

Only the relative mouse collection asserts button state. The absolute report
keeps a Windows-compatible descriptor shape but always sends an all-released
button byte, preventing a button from becoming stuck in one of Windows'
independently tracked top-level HID collections.

Absolute pointer reports target the Windows primary display. Relative reports
remain subject to Windows pointer speed/acceleration. Text is intentionally
limited to the firmware's US-ASCII keyboard layout because USB HID does not
provide a portable Unicode text operation.

## Install and flash

PowerShell downloads the latest Arduino CLI only from Arduino's official
GitHub release, verifies its official SHA-256 checksum, and stores it under the
ignored project-local `.tools` directory:

```powershell
.\emulation\firmware\install-arduino-cli.ps1
.\emulation\firmware\flash.ps1
```

`flash.ps1` installs or upgrades to the latest official `arduino:avr` and
`Keyboard` packages, applies the Logitech USB properties as build arguments, compiles,
and uploads. It does not patch the installed AVR core. During upload the Leonardo briefly enters its
Arduino bootloader identity; COM3 can disappear and return as Windows
re-enumerates it.

The flash script auto-detects the `046D:C223` CDC port. For a stock, unflashed
Leonardo, or when multiple matching boards are attached, pass `-Port COM3`
explicitly.

Compile without touching the board:

```powershell
.\emulation\firmware\flash.ps1 -CompileOnly
```

## Go API

```go
ctx := context.Background()
device, err := emulation.Open(ctx, "COM3")
if err != nil { /* handle */ }
defer device.Close()

err = device.Do(ctx,
    emulation.TapKeys(emulation.Ctrl, emulation.MustKey('l')),
    emulation.WriteText("https://example.com"),
    emulation.TapKeys(emulation.KeyEnter),
    emulation.Pause(500*time.Millisecond),
    emulation.JumpTo(640, 400),
    emulation.MouseClick(emulation.ButtonLeft),
)
```

Direct methods are available for `KeyDown`, `KeyUp`, `Press`, `Type`, `Move`,
`MoveTo`, `MoveAbsolute`, `ButtonDown`, `ButtonUp`, `Click`, `DoubleClick`,
`Scroll`, `ReleaseKeyboard`, `ReleaseMouse`, `ReleaseAll`, and `CycleUSB`.

The example is a non-input health check by default. Pass `-demo` explicitly to
let it open Notepad and type a marker:

```powershell
go run .\cmd\emulation
go run .\cmd\emulation -demo
go run .\cmd\emulation -cycle 1s
```

The example locates the CDC port by `046D:C223`, so it continues to work when
Windows assigns a new COM number after flashing. Pass `-port COM5` to select a
specific port when more than one matching device is connected.

## Tests

`go test ./...` exercises framing/CRC errors, negative acknowledgements,
timeouts, disconnects, cancellation cleanup, text/key validation, chords,
mouse buttons, relative and absolute movement encoding, two-axis scrolling,
USB cycling, concurrency, and action composition with an in-memory transport.
It never emits real input.

An explicitly enabled smoke test checks the attached board with a reversible
Shift press and one-unit pointer move/inverse:

```powershell
$env:TOOLBOX_EMULATION_HARDWARE = '1'
go test -tags hardware -run TestLeonardoHardwareSmoke .\emulation
```
