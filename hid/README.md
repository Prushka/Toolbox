# Arduino Leonardo HID library

The `hid` package controls an Arduino Leonardo from Go on Windows. The Leonardo appears as a
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
| Use separate absolute and relative mouse reports | Supports pixel jumps, accelerated relative motion, five buttons, and two wheel axes; standard report ID 1 owns buttons 1-3, while report ID 4 owns buttons 4-5. |
| Allow one acknowledged command in flight | Matches the firmware's single-threaded loop, bounds AVR memory, and makes response matching deterministic. |
| Use typed Go calls and composable actions | Avoids an AHK-style command language while retaining explicit contexts, errors, and reusable workflows. |
| Validate on both host and firmware | Go returns mistakes early; firmware remains safe when driven by another serial client. |
| Fail toward released input | Cleanup, CDC line-state handling, startup reset, and watchdog behavior reduce stuck-key/button risk at the cost of releasing unrelated held state after an uncertain failure. |
| Limit text to US-ASCII | Matches Arduino Keyboard's portable model; arbitrary Unicode would require host-layout-specific logic. |
| Scope host integration to Windows | Keeps COM discovery and primary-display mapping focused; non-Windows stubs return `ErrUnsupported`. |

| Path | Purpose |
| --- | --- |
| `hid/client.go` | Command client, validation, cleanup, and input API |
| `hid/actions.go` | Composable sequential workflow actions |
| `hid/key.go` | Arduino `Keyboard` key values and aliases |
| `hid/protocol.go` | Framing, CRC, opcodes, and response decoding |
| `hid/serial_windows.go` | Windows COM-port opening and handshake |
| `hid/ports_windows.go` | VID/PID-based USB CDC discovery |
| `hid/screen_windows.go` | Primary-display cursor coordinates |
| `hid/firmware/leonardo/leonardo.ino` | Firmware and custom mouse HID descriptor |
| `hid/firmware/install-arduino-cli.ps1` | Verified official Arduino CLI installer |
| `hid/firmware/flash.ps1` | Board package install, compile, and upload |
| `cmd/hid/*/main.go` | Focused status, keyboard, mouse, action, and USB-cycle examples |
| `cmd/hid/internal/example` | Shared example port discovery, opening, and cleanup |

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

The current firmware reports version `1.8`. The sketch includes Arduino's
`Keyboard` and low-level `HID` libraries and appends custom mouse descriptors:

- Standard report ID `1`: buttons 1-3, signed 8-bit X/Y, and vertical wheel.

- Report ID `3`: absolute pointer with button fields held at zero and unsigned
  16-bit X/Y in `0..32767`.

- Report ID `4`: buttons 4-5 and horizontal AC Pan in `-127..127`; its X/Y and
  vertical-wheel fields remain zero.

Movement, wheels, and buttons therefore belong to the same emulated pointer in
both Windows and raw-input applications. Firmware 1.8 advertises absolute
pointer, batched-linear-mouse, and batched-relative-mouse capabilities.
`MoveTo` uses one absolute report. `MoveToWindow` batches full-size reports
only while clamping a raw-input pointer to the client edge, then uses
individually acknowledged small reports for exact target alignment.

The relative button mask is kept in RAM and rolled back when a HID report fails.
Standard mouse reports bypass Arduino's void-returning `Mouse` calls so a USB
send failure cannot be acknowledged as success.
Keyboard HID failures call `Keyboard.releaseAll()`. Startup emits an all-released
state. Keyboard payloads use Arduino Keyboard 1.0.7 values: printable US-ASCII,
modifiers, navigation, keypad keys, Menu, and F13-F24. Both Go and firmware
reject unsupported values, duplicates, and more than six non-modifier keys.

At the audited Arduino AVR 1.8.8 and Keyboard 1.0.7 versions, the sketch uses
7,610 bytes (26%) of flash and 337 bytes (13%) of SRAM on a Leonardo. Package
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
| `0x21` | Legacy absolute move (unsupported by firmware 1.3) | little-endian `uint16 x`, `uint16 y` |
| `0x22` | Mouse button down | five-bit mask |
| `0x23` | Mouse button up | five-bit mask |
| `0x24` | Release mouse | empty |
| `0x25` | Batched linear move | 1..32 `dx, dy` signed-byte pairs, each axis in `[-4, 4]` |
| `0x26` | Batched relative move | 1..32 `dx, dy` signed-byte pairs |
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
mouse, absolute mouse, horizontal wheel, USB detach/attach, batched linear
mouse movement, and batched relative mouse movement.

## Go client and concurrency

`Open` is Windows-only. It opens the COM port, starts a reader goroutine, and
requires a successful Info handshake. `NewClient` is available for tests and
custom transports. A custom transport must support concurrent Read, Write, and
Close, and Close must promptly unblock pending I/O.

The client is safe for concurrent command-level use:

- A one-token channel serializes command acquisition, writes, sequence assignment,
  and response matching.
- One reader goroutine owns stream parsing.
- A response channel transfers decoded frames to the waiting command.
- Reader errors use an `RWMutex`.
- `atomic.Bool` and `sync.Once` make shutdown idempotent for concurrent
  `Close` calls.
- Context cancellation applies while waiting for the token, writing, and acknowledgement.
  Cleanup uses a bounded 250 ms context.

Command serialization does not make a multi-command workflow atomic. A
concurrent `Press` and `KeyDown` can interleave at command boundaries. `Do`
preserves action order for its caller but does not add cross-goroutine
isolation. Use one owner goroutine or an external mutex when exclusive workflow
ordering matters.

The default write-and-acknowledgement timeout is two seconds and the default press/click
hold time is 20 ms:

```go
device, err := hid.Open(ctx, "COM5",
    hid.WithTimeout(3*time.Second),
    hid.WithTapDelay(15*time.Millisecond),
)
```

`Press` releases a chord in reverse order, and `Click` releases its requested
button mask. If a down command cannot be confirmed, cleanup resets the relevant
keyboard or mouse state because the host cannot know whether firmware applied
the timed-out command. A timeout or cancellation during a blocked write closes
CDC to prevent a partial frame from corrupting a later command. `Close` attempts
release once, closes CDC, and returns both release and transport errors consistently
to repeated or concurrent callers. Applications should propagate these errors.
Physical unplug is safe: Windows removes the HID collections, and the firmware
watchdog/startup release clears locally held state.

## Input API

Direct methods cover:

- Keyboard: `KeyDown`, `KeyUp`, `Press`, `Type`, `ReleaseKeyboard`.
- Pointer: `Move`, `MoveAbsolute`, Windows `MoveTo`, `MoveToRelative`,
	`ClickAt`, `MoveToWindow`, `ClickAtWindow`, `ClickPreparedWindow`,
	`CursorPosition`.
- Buttons: `ButtonDown`, `ButtonUp`, `Click`, `DoubleClick`, `ReleaseMouse`.
- Wheel: `Scroll` with independent vertical and horizontal values.
- Lifecycle: `Info`, `Ping`, `ReleaseAll`, `CycleUSB`, `Close`.

Mouse button constants map directly to the five-bit report: left `0x01`, right
`0x02`, middle `0x04`, back `0x08`, and forward `0x10`. Multiple buttons may be
passed together and are combined into one report mask.

Large relative movements and scrolls split into signed 8-bit reports.
When firmware advertises batched linear mouse movement, `MoveLinear` packs up
to 32 four-count-or-smaller relative reports into one acknowledged command.
The firmware validates every pair before sending any HID report. Older
firmware and custom `NewClient` transports retain one command per report.
`MoveAbsolute` remains available for compatible older firmware that advertises
absolute HID support. Windows `MoveTo` uses absolute movement;
`MoveToRelative` reaches a primary-display pixel with relative HID reports and
cursor feedback, then allows raw-input consumers to drain the final movement
reports before returning.
`MoveToWindow` negotiates relative-mouse capabilities before its first
calibration report, so callers using `NewClient` receive the same batching
behavior as callers using `Open`.
`MoveToWindow` aligns a foreground raw-input pointer and the Windows cursor.
It observes stable physical cursor feedback at the virtual-desktop calibration
edge. Windows
acceleration means the later raw-input alignment phase does not share its
client-coordinate endpoint with the OS cursor, so absolute alignment follows
that phase and may reassert only the same button-free position after one second
of abnormal cursor delay. This remains bounded to seven seconds and never
repeats a click. Callers can align before pressing a key that enters a placement
mode.
`ClickAtWindow` aligns and clicks in one call. It verifies that another physical
mouse did not move between alignment and button-down, recalibrating and retrying
when needed. Targeted `ClickAt` adds a rendered-frame settlement period before
its Arduino click.
`ClickPreparedWindow` performs no movement: it clicks only when the live cursor
still matches the exact target prepared by `MoveToWindow`, returning
`ErrWindowPointerMoved` after another mouse changes that position.
`MoveToWindow` uses small HID deltas that stay in Windows' 1:1 range, preventing
pointer acceleration from separating the OS cursor from raw-input applications.

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
    hid.TapKeys(hid.GUI, hid.MustKey('r')),
    hid.Pause(150*time.Millisecond),
    hid.WriteText("notepad"),
    hid.TapKeys(hid.KeyEnter),
    hid.MouseClick(hid.ButtonLeft),
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
.\hid\firmware\install-arduino-cli.ps1
.\hid\firmware\flash.ps1
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
.\hid\firmware\flash.ps1 -CompileOnly
```

Downloaded CLI versions remain under ignored `.tools/`. Arduino CLI manages
the AVR core, libraries, and build cache in its normal per-user data locations;
the scripts do not overwrite Windows system files.

## Example programs

Each subdirectory under `cmd/hid` is an independent command. The status
example performs only discovery, Info, and health checks. Examples that send
real input or cycle USB require `-run`. The commands use Zerolog for structured
errors and warnings, while successful command results remain plain stdout:

```powershell
go run .\cmd\hid\status
go run .\cmd\hid\status -port COM5
go run .\cmd\hid\keyboard -run
go run .\cmd\hid\mouse -run
go run .\cmd\hid\actions -run
go run .\cmd\hid\cycleusb -run -duration 1s
```

See `cmd/hid/README.md` for example-specific flags and extension guidance.

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
go vet ./hid ./cmd/hid/...
```

The physical smoke test is opt-in. It uses a reversible Shift press, one-unit
move/inverse, an absolute move back to the original cursor, CRC rejection,
partial-frame expiry, and final `ReleaseAll`:

```powershell
$env:TOOLBOX_HID_HARDWARE = '1'
$env:TOOLBOX_HID_PORT = 'COM5' # optional
go test -race -tags hardware -run TestLeonardoHardwareSmoke -v .\hid
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
