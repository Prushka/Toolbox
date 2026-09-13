# Toolbox Agent Guide

Read this file before changing the repository. Treat the code and tests as the
source of truth. Update this guide in the same change whenever a durable fact
changes: package contracts, API decisions, verification steps, hardware
behavior, or known limitations. Replace stale facts instead of appending a
changelog. Do not record timestamps, process IDs, or command output.

## Project

- Repository: `C:\Users\dan\GolandProjects\Toolbox`
- Module: `github.com/Prushka/Toolbox`
- Go version: 1.26
- Consumer: `C:\Users\dan\GolandProjects\Toolbox-Apps` uses this module through
  a `replace` directive, so changes here are consumed immediately. Verify both
  repositories after touching shared behavior.

Toolbox is a library of generic primitives. Application workflows, retry
policy, screen semantics, and game-specific decisions belong in consumers.
Keep additions generic, documented, and covered by offline tests.

## Layout

- `automation/`: Windows observation and window management. Capture
  (`capture_windows.go`), templates and tolerance search (`image.go`),
  correlation scoring (`match.go`), pixel search and frame-stability waits
  (`search.go`), color classes (`color.go`), bitmap statistics (`stats.go`),
  windows and DPI (`window_windows.go`), display modes and advanced color
  (`display_windows.go`, `display_color_windows.go`), input monitoring and
  polling (`monitor*.go`, `input*.go`), timing (`timing*.go`), power requests
  (`power*.go`), continuous frame progress (`frame_progress.go`), and pinned
  process lifecycle (`process_handle*.go`). Non-Windows stubs return `ErrUnsupported`.
- `hid/`: Arduino Leonardo keyboard/mouse over USB CDC. Framing and CRC
  (`protocol.go`), command client (`client.go`), Windows cursor alignment
  (`screen_windows.go`), actions (`actions.go`), port discovery
  (`ports_windows.go`), firmware (`firmware/leonardo/leonardo.ino`, version
  1.8).
- `cmd/automation`, `cmd/hid`: focused runnable examples. `cmd/pip_transparent`
  is a separate utility.
- `automation/README.md` and `hid/README.md` are the user-facing references;
  keep them consistent with this guide.

## Automation Contracts

- `automation` never synthesizes keyboard or mouse input and never injects
  into processes. Input goes through `hid`.
- Coordinates: `Rect` is half-open; a zero `Rect{}` means the whole bitmap or
  client area. Window helpers use client coordinates. Call `SetDPIAware` at
  process start.
- Capture: `CaptureVisible` reads desktop pixels with `BitBlt` and is the only
  method that reliably reflects a GPU-rendered game. `PrintWindow` can report
  success with a stale frame. Every capture allocates a new `Bitmap`; a full
  1920x1080 client capture costs roughly 5 to 15 ms.
  GDI capture and pixel reads remain on one OS thread through DC cleanup;
  captures flush pending GDI drawing before reading DIB memory. Power-request
  context storage includes the complete native REASON_CONTEXT union on both
  32-bit and 64-bit Windows.
  On the BTD6 test machine, temporary HDR mode changes can disconnect every
  active display path; subsequent visible captures are black and restoring the
  prior mode can fail until the desktop is restored. Keep the active display
  mode during further live acceptance. A fallback monitor is not proof of a
  usable visible desktop, and changing resolution is not a valid workaround.
- `RequireActiveDisplay` checks current active display paths and their target
  availability without querying HDR capabilities. No available target, or loss
  of console access, returns `ErrDisplayUnavailable`; a sleeping monitor that
  remains connected can still report an available path. This is not a backlight
  or application-rendering check. `CaptureOptions.RequireActiveDisplay` opts
  window captures and searches into checks before and after capture, discarding
  a bitmap if the display disappears. Default capture behavior is unchanged.
- Template search has two modes:
  - `SearchTemplate` is the AutoHotkey-style per-channel tolerance match. It
    is exact and fast but fails when display tone mapping, anti-aliasing, or
    dimming shifts pixel values; raising `Variation` toward 120 makes it match
    unrelated content.
  - `SearchTemplateScore` uses correlation over opaque pixels, centering each
    color channel and accumulating in float64 to handle low spatial contrast.
    Uniform brightness offsets and common contrast scaling preserve scores;
    nonlinear HDR/SDR tone mapping still requires consumer fixture validation.
    Default sampled scanning returns the best verified candidate and can miss
    narrow peaks. `Exhaustive` scores every location for a guaranteed best match;
    use tight regions to bound its cost. `MinScore` is application-specific.
    Spatially flat templates have no contrast (`HasContrast`) and cannot be
    scored. Correlation data is compiled lazily and safely shared; tolerance-only
    searches do not allocate it. Scoring never accepts NaN thresholds or
    overflowing locations. Rotation and scale changes are not normalized.
- `ColorRange` classifies pixels by hue, saturation, and value. Hue survives
  desktop tone mapping far better than an RGB box, so consumers should express
  pixel signatures this way. `Bitmap.Fraction`, `Count`, and `Mean` report
  region statistics for such classes.
- `DisplayColors` reports whether Windows advanced color is enabled and
  the SDR white level. Match its `DeviceName` to `Monitor.DeviceName`; output
  order does not imply primary status. On supported Windows versions, ColorMode
  distinguishes "hdr", "wcg", and "sdr"; empty means unknown. Advanced color
  can remain enabled for WCG when HDR is off, so it is not an HDR-only flag.
  Buffer sizing retries when display
  topology changes during a query. Live BTD6 consumer validation covers HDR at
  a 420-nit SDR white level and HDR-off WCG at 80 nits. Captured 8-bit application
  values depend on composition mode; consumers must not rely on exact RGB values.
- Frame waits: `WaitForBitmapChange` proves an application is presenting new
  frames; `WaitForBitmapTransition` requires a change after an action and then
  stability. Both reject captures that complete after context cancellation.
  Bitmap similarity and color saturation/value comparisons reject NaN limits
  instead of allowing unordered floating-point comparisons to pass.
- `FrameProgress` tracks continuously observed unchanged bitmaps using monotonic
  timestamps. Invalid frames, non-increasing time, or observation gaps exceeding
  `MaxGap` reset proof. The consumer owns desktop usability, screen semantics,
  duration thresholds, and restart policy. The tracker is owned by one goroutine
  and retains immutable capture data.
- `OpenProcess` pins a Windows process handle with only query and synchronization
  rights, so observation does not require permission to terminate. `Terminate`
  requests termination permission only when called and uses `CompareObjectHandles`
  to verify that the new handle identifies the pinned kernel object. Cancellation,
  permission denial, unavailable identity comparison, and mismatched objects
  prevent termination. `Close` only releases the observation handle.
  `Window.IsHung` exposes Windows' message-pump assessment; false does not prove
  GPU rendering progress. `Window.GhostWindow` optionally resolves Windows'
  replacement through reciprocal User32 ghost/HWND mappings, never title or
  geometry. Those undocumented exports are queried dynamically; missing exports
  return `ErrUnsupported` and stale mappings return `ErrNotFound`. Consumers must
  still verify the original PID, foreground, visibility, and desktop usability.
  Tests terminate only disposable children, never the BTD6 consumer.
- `Window.EnsureActive` rejects cancellation before native window access and
  never reports an already-active window as successful for a canceled context.
- `WaitUntil` evaluates immediately, then on a ticker, and stops on
  cancellation. Contexts and callbacks are validated rather than panicking.
  `IsContextCancellation` recognizes cancellation-only error trees and rejects
  errors joined with cleanup failures; consumers use it when stopping workers.
  `InputMonitor.Close` joins native shutdown and reports its final cleanup
  result even if the done notification races the close-command acknowledgement.

## HID Contracts

- One acknowledged command is in flight at a time; the client serializes
  commands and matches responses by sequence number. Cleanup releases input
  with a bounded 250 ms context. `Press` and `Click` release in reverse order.
  `Close` attempts release only once across concurrent callers and preserves
  both release and transport-close errors on repeated calls. Consumers must
  propagate cleanup failures instead of counting the session as successful.
  Command deadlines include writes. Cancellation or timeout during a blocked
  write closes the port so an incomplete protocol frame cannot be followed by
  another command. Custom transports must unblock Read and Write when closed.
- Keyboard values are Arduino `Keyboard` codes (`hid.Key`), not Windows
  virtual keys. `automation.Key` is a Windows virtual key for polling only.
- Pointer movement:
  - `MoveTo` sends one absolute report to a primary-display pixel and waits
    for cursor feedback. It takes roughly 30 ms and is the right call for
    application menus that follow the Windows cursor.
  - `MoveToWindow` aligns a raw-input application's independent pointer by
    clamping it to the client edge with batched reports, then walking to the
    target with individually acknowledged four-count reports, then jumping the
    Windows cursor with an absolute report. It costs well over a second for a
    distant target and is only required where an application ignores the
    Windows cursor. `ClickPreparedWindow` clicks only if the cursor still sits
    on the prepared target.
    Subsequent moves may reuse the calibrated raw position when cursor feedback
    and client origin match; an identical prepared target needs no new reports.
    `ResetWindowPointer` invalidates this cache when a consumer starts a new
    raw-input session. Absolute-only menu moves preserve the raw position.
  - Batched four-count reports were rejected by a live raw-input consumer
    during exact alignment; do not batch the final alignment phase.
- Firmware 1.8 advertises keyboard, relative and absolute mouse, horizontal
  wheel, USB detach, and batched linear and relative movement. Reflash with
  `hid/firmware/flash.ps1`; the board enumerates as a Logitech-compatible
  composite device with one COM port.

## Verification

```powershell
gofmt -l .
go vet ./...
go test ./... -count=1
go test -race ./... -count=1
```

Windows integration tests run in the default suite. Hardware tests are
opt-in: `TOOLBOX_HID_HARDWARE=1` with optional `TOOLBOX_HID_PORT` enables the
Leonardo smoke test, which moves the real cursor.

## Known Limitations

- `MoveTo` targets the primary display only. Multi-monitor absolute movement
  is not implemented.
- `PrintWindow` capture of DirectX or Vulkan surfaces is unreliable; prefer
  visible capture and keep the target window unobscured.
- A disconnected desktop may expose a `WinDisc` fallback monitor while
  `DisplayColors` returns `ErrNotFound` because there are no active display
  paths. Visible capture can then be black even though window enumeration
  succeeds. Consumers must require usable visible application state before input.
- Correlation scoring is not rotation or scale invariant; consumers scale
  templates to the live viewport before searching.
