# automation examples

Each directory below is an independent `main` package. This layout keeps an
example focused and lets future examples be added without turning one command
into a large switch statement. The shared `internal/example` package contains
only flag parsing and window selection helpers; it is not part of the public
`automation` API.

Run every command from the repository root with `go run`:

```powershell
go run ./cmd/automation/windows -h
```

The examples use documented `automation` APIs only. They do not send keyboard
or mouse input. On non-Windows builds, Windows-specific examples return
`automation.ErrUnsupported`. Normal results are printed to stdout; operational
failures are timestamped Zerolog JSON events on stderr.

## Commands

| Command | Demonstrates | Side effects |
| --- | --- | --- |
| `capture` | Screen, whole-window, and client-region capture; PNG output; visible/PrintWindow/auto methods | Writes a PNG. |
| `pixel-search` | Client-relative color search and independent RGB tolerance | None. |
| `image-search` | Compiled image templates, AHK-style options, and cancellable polling | None. |
| `windows` | Window enumeration, selectors, geometry, state, and optional process paths | Process-path lookup opens query-limited handles. |
| `app-toggle` | Toggle an existing window or start a command when no match exists | May minimize/activate a window or start a process. |
| `displays` | Virtual desktop, monitor work areas, current primary mode, and available modes | None. |

## Shared window selectors

Commands that operate on a window accept these selectors. All supplied
selectors must match. The comparisons are case-insensitive.

| Flag | Meaning |
| --- | --- |
| `-title` | Exact window title. |
| `-title-contains` | Window-title substring. |
| `-class` | Exact Win32 window class. |
| `-process` | Process basename such as `example-app.exe`, or a full path. |
| `-pid` | Process ID. |
| `-visible` | Require a window Windows reports as visible. Defaults vary by command; use `-h` to inspect the default. |

`capture`, `pixel-search`, and `image-search` use the foreground window when
no selector is supplied. `app-toggle` requires a selector so it cannot act on
an arbitrary foreground window. `windows` lists all visible windows when no
selector is supplied; pass `-visible=false` to include hidden windows.

Coordinate rectangles use `left,top,right,bottom` and are half-open, exactly
like `automation.Rect`. For example, `100,50,500,250` has width 400 and height
200. The `-region` flags are client-relative; use the `capture -screen` flag
for physical desktop coordinates.

## Capture

Capture the client area of the foreground window:

```powershell
go run ./cmd/automation/capture -output active-client.png
```

Capture an explicit client region from a window selected by process name:

```powershell
go run ./cmd/automation/capture `
  -process example-app.exe `
  -region "120,80,520,320" `
  -output app-region.png
```

Capture physical screen coordinates from the visible virtual desktop:

```powershell
go run ./cmd/automation/capture `
  -screen "0,0,1920,1080" `
  -output desktop.png
```

For a conventional obscured desktop window, the explicit `PrintWindow` path is
available. It can block and can produce blank/stale results for GPU-rendered
applications, so visible capture is the default.

```powershell
go run ./cmd/automation/capture `
  -process notepad.exe `
  -method print `
  -client=false `
  -output notepad-window.png
```

`-method` accepts `visible`, `print`, and `auto`. `auto` only falls back when
`PrintWindow` reports a failure; it cannot detect an all-black but successful
result. `-screen` is intentionally restricted to `visible` capture. `-region`
is always client-relative and therefore cannot be combined with `-client=false`.

## Pixel search

Search the foreground window's whole client area for a color:

```powershell
go run ./cmd/automation/pixel-search -color 5CE100 -tolerance 30,30,2
```

Search only a client-relative application region. A single tolerance value applies to
all channels; `r,g,b` uses independent per-channel values.

```powershell
go run ./cmd/automation/pixel-search `
  -process example-app.exe `
  -region "1450,325,1521,350" `
  -color 5CE100 `
  -tolerance 30,30,2
```

The command prints the first matching client coordinate. A miss is successful
and prints `color not found`.

## Image search

Search once using a compiled PNG/JPEG/GIF/BMP/TIFF template:

```powershell
go run ./cmd/automation/image-search `
  -process example-app.exe `
  -image assets/ready.png `
  -variation 42
```

Wait up to 20 seconds, checking at a bounded interval. Press Ctrl+C to cancel.

```powershell
go run ./cmd/automation/image-search `
  -process example-app.exe `
  -image assets/ready.png `
  -region "1300,250,1650,500" `
  -variation 42 `
  -timeout 20s `
  -interval 16ms
```

Use `-transparent` with a six-digit `RRGGBB` color, or provide the AHK-style
image argument through `-spec`:

```powershell
go run ./cmd/automation/image-search `
  -process example-app.exe `
  -spec '*42 *TransBlack *w100 *h-1 "img\target.png"'
```

`-spec` cannot be combined with `-image`, `-variation`, `-transparent`,
`-width`, or `-height`; this prevents ambiguous configuration. The command
loads and compiles the template once, then reuses it through all polling
iterations.

## Window inspection and toggling

List visible windows with their handles, PID, effective DPI, state, geometry,
title, and class:

```powershell
go run ./cmd/automation/windows -title-contains "notepad" -process-path
```

By default it prints up to 25 matches. Use `-limit 0` for all matches. Process
paths are optional because retrieving them opens a query-limited process handle.

Toggle an existing Notepad window, or start Notepad when none matches:

```powershell
go run ./cmd/automation/app-toggle `
  -process notepad.exe `
  -command notepad.exe
```

Arguments can be supplied with repeated `-arg` flags:

```powershell
go run ./cmd/automation/app-toggle `
  -title-contains "Report" `
  -command notepad.exe `
  -arg "C:\\work\\report.txt"
```

The command does not wait for a newly started process to create a window. On an
existing window, toggle means: activate when minimized, minimize when already
foreground, otherwise activate. Foreground activation remains subject to normal
Windows focus-stealing rules.

## Display inspection

Inspect the virtual desktop, connected monitor work areas, and current primary
display mode:

```powershell
go run ./cmd/automation/displays
```

Print available primary-display modes without changing them:

```powershell
go run ./cmd/automation/displays -modes -limit 0
```

The example intentionally does not expose display-mode changes. `automation`
does provide `SetDisplayMode` and `RestoreDisplayMode` for callers that make
that system-wide choice explicitly.

## Verification

```powershell
go test ./cmd/automation/...
go vet ./cmd/automation/...
```

Run `go run ./cmd/automation/<command> -h` for the complete flags for any
specific example.
