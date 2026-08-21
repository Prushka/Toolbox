# Arduino Leonardo HID emulation

`emulation` controls an Arduino Leonardo from Go on Windows. The Leonardo appears as a
USB keyboard and mouse, while its native USB CDC interface carries commands
from the Go process. No virtual input driver is installed: input reports
originate from the ATmega32U4 USB device.

This package is for learning, controlled compatibility testing, and local
automation. It is not an anti-detection mechanism, an authentication system,
or a replacement for a genuine Logitech product.

## Architecture

```text
Go program
  |
  | framed commands and acknowledgements over USB CDC (COMx)
  v
Arduino Leonardo / ATmega32U4 firmware
  |
  +--> keyboard HID collection -> Windows HID keyboard stack
  +--> mouse HID collections    -> Windows HID mouse stack
```

Serial and HID are interfaces on one physical USB device. They are not separate
physical peripherals and cannot have independent USB identities.

Key design decisions are:

| Decision | Reason and tradeoff |
| --- | --- |
| Generate input on the Leonardo | Windows uses its built-in USB HID stack; no host input-emulation driver is needed. |
| Keep CDC, keyboard, and mouse in one composite device | One cable carries commands and HID reports, but all interfaces share one USB identity and the CDC topology remains observable. |
| Apply USB identity through compile properties | Avoids patching Arduino core files or `boards.txt`; the result is compatible identification, not an exact Logitech descriptor clone. |
| Use separate absolute and relative mouse reports | Supports pixel jumps, accelerated relative motion, five buttons, and two wheel axes; only the relative report owns buttons to avoid split state. |
| Allow one acknowledged command in flight | Matches the firmware's single-threaded loop, bounds AVR memory, and makes response matching deterministic. |
| Use typed Go calls and composable actions | Avoids an AHK-style command language while retaining explicit contexts, errors, and reusable workflows. |
| Validate on both host and firmware | Go returns mistakes early; firmware remains safe when driven by another serial client. |
| Fail toward released input | Cleanup, CDC line-state handling, startup reset, and watchdog behavior reduce stuck-key/button risk at the cost of releasing unrelated held state after an uncertain failure. |
| Limit text to US-ASCII | Matches Arduino Keyboard's portable model; arbitrary Unicode would require host-layout-specific logic. |
| Scope host integration to Windows | Keeps COM discovery and primary-display mapping focused; non-Windows stubs return `ErrUnsupported`. |

| Path | Purpose |
| --- | --- |
| `emulation/client.go` | Command client, validation, cleanup, and input API |
| `emulation/actions.go` | Composable sequential workflow actions |
| `emulation/key.go` | Arduino `Keyboard` key values and aliases |
| `emulation/protocol.go` | Framing, CRC, opcodes, and response decoding |
| `emulation/serial_windows.go` | Windows COM-port opening and handshake |
| `emulation/ports_windows.go` | VID/PID-based USB CDC discovery |
| `emulation/screen_windows.go` | Primary-display cursor coordinates |
| `emulation/firmware/leonardo/leonardo.ino` | Firmware and custom mouse HID descriptor |
| `emulation/firmware/install-arduino-cli.ps1` | Verified official Arduino CLI installer |
| `emulation/firmware/flash.ps1` | Board package install, compile, and upload |
| `cmd/emulation/*/main.go` | Focused status, keyboard, mouse, action, and USB-cycle examples |
| `cmd/emulation/internal/example` | Shared example port discovery, opening, and cleanup |

## USB identity and compatibility

The firmware is built with:

| Property | Value |
| --- | --- |
| Vendor ID | `046D` |
| Product ID | `C223` |
| Manufacturer string | `Logitech` |
| Product string | `Logitech G15 Gaming Keyboard` |

`046D:C223` is the Logitech VID/PID pair used by the existing reference sketch. The
public USB ID database labels it `Logitech, Inc. G11/G15 Keyboard / USB Hub`:

- <https://github.com/usbids/usbids/blob/master/usb.ids>
- <https://docs.arduino.cc/hardware/leonardo/>

This is best-effort Logitech-compatible identification, not a bit-for-bit clone
of a G11, G15, or Logitech mouse. A Leonardo exposes a composite device with
CDC plus custom HID collections, so Windows can distinguish it from a genuine
product by descriptors, interface layout, physical USB path, and container ID.
One Leonardo also cannot expose different VID/PID pairs for keyboard and mouse
interfaces. Do not select devices by VID/PID alone if a real `046D:C223` device
may be connected.

With the current firmware, Windows has enumerated the board as a USB Composite
Device, USB Input Device, HID Keyboard Device, two HID-compliant mouse
collections, and a USB Serial Device. The command port is currently a normal
Windows COM port and may receive a different number after upload or USB cycle.

The descriptors do not advertise the word `emulation`, but the implementation does
not hide its composite topology or bypass endpoint inspection.

## Prerequisites

- Arduino Leonardo or another ATmega32U4 board using the Leonardo USB core.
- USB data cable, Windows, PowerShell, and Go 1.26 or newer for this repository.
- Permission to open the board's CDC COM port.

Go and firmware use 115200 as the nominal serial speed. Native USB CDC does not
use a UART baud rate, but this conventional value keeps tooling predictable.

## Firmware

The current firmware reports version `1.1`. The sketch includes Arduino's
`Keyboard` and low-level `HID` libraries and appends a custom mouse descriptor
with two top-level collections:

- Report ID `3`: five-button absolute pointer, 16-bit X/Y, logical range
  `0..32767`.
- Report ID `4`: five-button relative pointer with signed 8-bit X/Y,
  vertical wheel, and horizontal AC Pan, each in `-127..127`.

Only report ID `4` owns button state. Absolute reports always send a zero button
byte, preventing Windows from tracking one button in two independently managed
collections and leaving it logically stuck across absolute/relative movement.

The relative button mask is kept in RAM and rolled back when a HID report fails.
Keyboard HID failures call `Keyboard.releaseAll()`. Startup emits an all-released
state. Keyboard payloads use Arduino Keyboard 1.0.7 values: printable US-ASCII,
modifiers, navigation, keypad keys, Menu, and F13-F24. Both Go and firmware
reject unsupported values, duplicates, and more than six non-modifier keys.

At the audited Arduino AVR 1.8.8 and Keyboard 1.0.7 versions, the sketch uses
7,134 bytes (24%) of flash and 322 bytes (12%) of SRAM on a Leonardo. Package
updates can change those figures slightly.

The firmware loop is single-threaded: parse one complete CDC frame, execute one
command, write one acknowledgement, then process the next frame. Safeguards:

- 64-byte maximum payload.
- 250 ms partial-frame expiry.
- CRC-8/SMBUS validation before command execution.
- Explicit bad-payload, bad-checksum, bad-version, unknown-command, and
  HID-failure statuses.
- 30-second watchdog releasing all keyboard and mouse state after the last
  valid command.
- Immediate release and parser reset when both CDC DTR/RTS lines drop.

The USB-cycle command releases input, acknowledges, flushes CDC output, waits
75 ms, clears the ATmega32U4 USB pull-up, waits the requested 250 ms to 30 s,
and calls `USBDevice.attach()`. The Go `Client` is closed after acknowledgement;
Windows must enumerate a new COM port before `Open` is called again. Direct
`UDCON` detach is intentional because some Arduino AVR cores leave
`USBDevice.detach()` empty on Leonardo.

## CDC protocol

Frames are length-delimited and use protocol version `1`:

```text
request:  magic(1) version(1) sequence(1) opcode(1) length(1) payload(length) crc(1)
response: magic(1) version(1) sequence(1) status(1) length(1) payload(length) crc(1)
```

Request magic is `0xA5`; response magic is `0x5A`. CRC is CRC-8/SMBUS
(polynomial `0x07`, initial `0`, non-reflected) over the frame through
the payload. The one-byte sequence lets the client discard delayed responses
after a timeout.

| Opcode | Operation | Payload |
| --- | --- | --- |
| `0x01` | Ping | empty |
| `0x02` | Info/capability negotiation | empty |
| `0x10` | Key down | one or more key values |
| `0x11` | Key up | one or more key values |
| `0x12` | Release keyboard | empty |
| `0x13` | Type ASCII | 1..64 printable bytes |
| `0x20` | Relative move/scroll | `dx, dy, wheel, pan` signed bytes |
| `0x21` | Absolute move | little-endian `uint16 x`, `uint16 y` |
| `0x22` | Mouse button down | five-bit mask |
| `0x23` | Mouse button up | five-bit mask |
| `0x24` | Release mouse | empty |
| `0x30` | Release keyboard and mouse | empty |
| `0x31` | Detach and reattach USB | little-endian milliseconds |

Response statuses are:

| Status | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Unknown opcode |
| `2` | Invalid payload |
| `3` | HID report failure |
| `4` | Request checksum failure |
| `5` | Protocol version mismatch |

Info reports firmware major/minor, protocol version, capability bits, maximum
payload, and watchdog seconds. Current capabilities are keyboard, relative
mouse, absolute mouse, horizontal wheel, and USB detach/attach.

## Go client and concurrency

`Open` is Windows-only. It opens the COM port, starts a reader goroutine, and
requires a successful Info handshake. `NewClient` is available for tests and
custom transports.

The client is safe for concurrent command-level use:

- A one-token channel serializes command acquisition, writes, sequence assignment,
  and response matching.
- One reader goroutine owns stream parsing.
- A response channel transfers decoded frames to the waiting command.
- Reader errors use an `RWMutex`.
- `atomic.Bool` and `sync.Once` make shutdown idempotent for concurrent
  `Close` calls.
- Context cancellation applies while waiting for the token and acknowledgement.
  Cleanup uses a bounded 250 ms context.

Command serialization does not make a multi-command workflow atomic. A
concurrent `Press` and `KeyDown` can interleave at command boundaries. `Do`
preserves action order for its caller but does not add cross-goroutine
isolation. Use one owner goroutine or an external mutex when exclusive workflow
ordering matters.

The default acknowledgement timeout is two seconds and the default press/click
hold time is 20 ms:

```go
device, err := emulation.Open(ctx, "COM5",
    emulation.WithTimeout(3*time.Second),
    emulation.WithTapDelay(15*time.Millisecond),
)
```

`Press` releases a chord in reverse order, and `Click` releases its requested
button mask. If a down command cannot be confirmed, cleanup resets the relevant
keyboard or mouse state because the host cannot know whether firmware applied
the timed-out command. `Close` best-effort releases all input and closes CDC.
Physical unplug is safe: Windows removes the HID collections, and the firmware
watchdog/startup release clears locally held state.

## Input API

Direct methods cover:

- Keyboard: `KeyDown`, `KeyUp`, `Press`, `Type`, `ReleaseKeyboard`.
- Pointer: `Move`, `MoveAbsolute`, Windows `MoveTo`, `CursorPosition`.
- Buttons: `ButtonDown`, `ButtonUp`, `Click`, `DoubleClick`, `ReleaseMouse`.
- Wheel: `Scroll` with independent vertical and horizontal values.
- Lifecycle: `Info`, `Ping`, `ReleaseAll`, `CycleUSB`, `Close`.

Mouse button constants map directly to the five-bit report: left `0x01`, right
`0x02`, middle `0x04`, back `0x08`, and forward `0x10`. Multiple buttons may be
passed together and are combined into one report mask.

Large relative movements and scrolls split into signed 8-bit reports.
`MoveAbsolute` accepts normalized HID coordinates. Windows `MoveTo` converts
primary-display pixels using `GetSystemMetrics`; relative motion remains subject
to Windows pointer speed and acceleration.

`Type` validates the complete input before sending it, accepts printable
US-ASCII, and maps newline, carriage return, tab, and backspace to explicit
taps. CRLF becomes one Enter. Unicode is rejected because USB HID has no
portable Unicode text operation. A transport failure can still occur after an
earlier valid chunk was typed. Text follows the host's active keyboard layout.

`Key` constants match Arduino byte values. `RuneKey` returns a physical
lowercase US-ASCII key; use Shift for an individual uppercase key. `Type`
delegates printable characters to Arduino `Keyboard.write`.

The action API avoids an AHK-style string parser:

```go
err := device.Do(ctx,
    emulation.TapKeys(emulation.GUI, emulation.MustKey('r')),
    emulation.Pause(150*time.Millisecond),
    emulation.WriteText("notepad"),
    emulation.TapKeys(emulation.KeyEnter),
    emulation.MouseClick(emulation.ButtonLeft),
)
```

Available constructors are `HoldKeys`, `ReleaseKeys`, `TapKeys`,
`WriteText`, `MoveBy`, `JumpTo`, `MouseHold`,
`MouseRelease`, `MouseClick`, `Wheel`, `Pause`, and
`Repeat`. `Do` stops at the first error and attempts `ReleaseAll`
with bounded cleanup.

Firmware negative acknowledgements are returned as `*DeviceError`. Closed
clients return `ErrClosed`; Windows-only operations return `ErrUnsupported` on
other operating systems. Transport, timeout, framing, and context errors retain
their causes for `errors.Is`/`errors.As` checks.

## Install and flash

The scripts use official Arduino release endpoints. The CLI installer queries
the official Arduino CLI GitHub release, downloads the Windows 64-bit archive
and published `checksums.txt`, verifies SHA-256, and extracts it under the
ignored project-local `.tools` directory. Temporary cleanup refuses paths
outside the system temp directory.

```powershell
.\emulation\firmware\install-arduino-cli.ps1
.\emulation\firmware\flash.ps1
```

`flash.ps1` installs/upgrades the official `arduino:avr` core and
`Keyboard` library, compiles `arduino:avr:leonardo`, and uploads it with
the VID/PID and Logitech descriptor strings as build properties. It does not
patch `boards.txt`, modify the AVR core, install a Windows emulation driver,
edit the registry, or modify the Windows driver store. Arduino CLI caches are
development-tool state, not a runtime HID driver.

The script auto-detects exactly one PNP ID containing `VID_046D&PID_C223`.
Pass `-Port COM5` for an unflashed board, multiple matching devices, or a
changed COM number. Upload briefly enters the bootloader, so the runtime COM
port can disappear and return under a different number.

Compile without uploading:

```powershell
.\emulation\firmware\flash.ps1 -CompileOnly
```

Downloaded CLI versions remain under ignored `.tools/`. Arduino CLI manages
the AVR core, libraries, and build cache in its normal per-user data locations;
the scripts do not overwrite Windows system files.

## Example programs

Each subdirectory under `cmd/emulation` is an independent command. The status
example performs only discovery, Info, and health checks. Examples that send
real input or cycle USB require `-run`:

```powershell
go run .\cmd\emulation\status
go run .\cmd\emulation\status -port COM5
go run .\cmd\emulation\keyboard -run
go run .\cmd\emulation\mouse -run
go run .\cmd\emulation\actions -run
go run .\cmd\emulation\cycleusb -run -duration 1s
```

See `cmd/emulation/README.md` for example-specific flags and extension guidance.

## Tests and verification

Default tests use an in-memory `net.Pipe` and never emit real input. They cover
framing/CRC, payload limits, protocol errors, delayed/corrupt responses,
timeouts, disconnects, cancellation cleanup, idempotent close, key validation,
six-key rollover, chords, text chunking and normalization, Unicode rejection,
movement and wheel chunking, button masks, actions, USB cycling, and concurrent
command serialization.

```powershell
go test ./...
go test -race ./...
go vet ./emulation ./cmd/emulation/...
```

The physical smoke test is opt-in. It uses a reversible Shift press, one-unit
move/inverse, an absolute move back to the original cursor, CRC rejection,
partial-frame expiry, and final `ReleaseAll`:

```powershell
$env:TOOLBOX_EMULATION_HARDWARE = '1'
$env:TOOLBOX_EMULATION_PORT = 'COM5' # optional
go test -race -tags hardware -run TestLeonardoHardwareSmoke -v .\emulation
```

Do not enable the hardware test unattended; it moves the real cursor and sends
a real keyboard report.

## Safety and limitations

- CDC has no authentication. CRC detects accidental corruption, not a malicious
  sender. Keep the port local and trusted.
- A genuine Logitech device with the same VID/PID can confuse software that does
  not inspect the USB interface or container identity.
- This is one composite Leonardo, not two independent Logitech devices.
- Keyboard behavior follows Arduino's US-ASCII/active-layout rules; Unicode is
  outside this API.
- Absolute coordinates target the primary display only.
- USB detach timing and Windows re-enumeration depend on the host and device.
- Go provides command-level concurrency safety, not ownership or transactional
  isolation for overlapping high-level input workflows.

The design favors explicit protocol errors, bounded cleanup, all-released
startup/shutdown states, and observable Windows behavior over disguising the
board as a genuine commercial product.
