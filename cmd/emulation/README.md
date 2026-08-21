# Emulation examples

Each subdirectory is an independent Windows command. Shared board discovery,
opening, and cleanup live in `internal/example`, leaving each `main.go` focused
on the API being demonstrated.

| Example | Purpose | Generates input by default |
| --- | --- | --- |
| `status` | Discover, handshake, ping, and print firmware capabilities | No |
| `keyboard` | Chords, held modifiers, special keys, and US-ASCII text | Requires `-run` |
| `mouse` | Relative/absolute movement, wheels, click, and double-click | Requires `-run` |
| `actions` | Sequential `Do`, `Repeat`, pauses, keyboard, and pointer actions | Requires `-run` |
| `cycleusb` | Physical USB detach/attach and COM-port re-enumeration | Requires `-run` |

Run the non-input status example:

```powershell
go run .\cmd\emulation\status
go run .\cmd\emulation\status -port COM5
```

Run input examples explicitly:

```powershell
go run .\cmd\emulation\keyboard -run
go run .\cmd\emulation\keyboard -run -text "Custom US-ASCII text"
go run .\cmd\emulation\mouse -run
go run .\cmd\emulation\mouse -run -x 640 -y 400 -wheel -3 -click
go run .\cmd\emulation\actions -run -repeat 3
go run .\cmd\emulation\cycleusb -run -duration 1s
```

All commands accept `-port COM5`; when omitted, exactly one `046D:C223` CDC
port must be discoverable. Use `go run .\cmd\emulation\<name> -help` for all
flags. Input examples operate the real Windows desktop, so run them only while
attending the machine.

To add another example, create `cmd/emulation/<name>/main.go` and reuse
`cmd/emulation/internal/example.Open` and `Close` for consistent connection
handling.
