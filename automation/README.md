# automation

`automation` is a low-level Go package for screen observation, image/pixel
matching, read-only keyboard/mouse polling, window management, display
management, and timing facilities commonly used in AutoHotkey automation. It
is a primitive library rather than a workflow framework: callers own retry
policy, orchestration, and application-specific decisions.

The package deliberately has no keyboard or mouse synthesis, hotkey hooks,
cursor clipping, serial/HID integration, process memory access, code injection,
DLL loading into other processes, or security-bypass features. It does provide
read-only keyboard and mouse polling so callers can implement their own action
triggers.

The Windows implementation uses documented User32, GDI32, Kernel32, and WinMM
APIs. Ordinary visible capture uses the desktop device context and `BitBlt`; it
does not message the target window or open a target-process handle.
`CapturePrintWindow`, process-path queries, and process-priority changes are
explicit exceptions with normal documented Windows behavior:

- `CapturePrintWindow` sends a synchronous render request to the target window.
- `Window.ProcessPath`, and a `WindowQuery` with `Process`, open a
  query-limited process handle.
- `SetProcessPriority` opens a process handle with permission to change its
  priority class.

This is a small, passive-by-default footprint, not a promise of invisibility
or compatibility with every application. Protected, elevated, remote,
sandboxed, and GPU-rendered applications may refuse, restrict, blank, or alter
normal Windows APIs. The package does not attempt to work around those
boundaries.

The package builds on non-Windows platforms so code can be tested there.
Windows-specific window, capture, display, and process operations return
`ErrUnsupported` on those platforms. `BeginTimerResolution` is a no-op lease
outside Windows.

Call `SetDPIAware` at process startup, before creating any windows, when
physical-pixel accuracy is required. It requests per-monitor-v2 awareness
where available and falls back to the older process-wide setting. It is safe
to call concurrently and repeatedly.

## Quick start

```go
import (
    "context"
    "time"
    "github.com/Prushka/Toolbox/automation"
)

func observe(ctx context.Context) error {
	if err := automation.SetDPIAware(); err != nil {
		return err
	}

	target, err := automation.FindWindow(automation.WindowQuery{
		Process:     "ExampleApp.exe",
		VisibleOnly: true,
	})
	if err != nil {
		return err
	}

	// Compile/decode once when searching for the same image repeatedly.
	tpl, err := automation.LoadTemplate(`assets\ready.png`, automation.ImageSearchOptions{
		Variation: 42,
	})
	if err != nil {
		return err
	}

	return automation.WaitUntil(ctx, 16*time.Millisecond, func() (bool, error) {
		_, found, err := target.SearchTemplate(automation.Rect{}, tpl)
		return found, err
	})
}
```

`Window.SearchPixel`, `Window.SearchTemplate`, and `Window.CaptureRegion`
capture only the requested client-area region. Prefer those calls over a full
screen capture followed by a crop.

Runnable examples live in [`cmd/automation`](../cmd/automation/README.md).
They are organized as one focused command per directory and cover capture,
pixel/image search, input polling, window inspection/toggling, and display
inspection.

## Coordinates and data model

`Point` is an integer coordinate. Its space is determined by the API: screen
coordinates for `CaptureScreen`, `PixelColor`, and `CursorPosition`; client
coordinates for window-relative helpers such as `Window.Pixel`,
`Window.CaptureRegion`, and `Window.SearchTemplate`.

`Rect` is always half-open: `[Left, Right) x [Top, Bottom)`. Its width is
`Right-Left`, its height is `Bottom-Top`, and its right/bottom edges are not
included. This matches Go image rectangles and makes adjacent regions compose
without overlap. A zero `Rect{}` means "the whole available bitmap/client
area" in APIs that accept a search or capture region. Use
`InclusiveRect(x1, y1, x2, y2)` to port AHK coordinate pairs whose endpoints
are inclusive.

`RGB` represents an AHK-style `0xRRGGBB` color. `ColorTolerance` is an
independent tolerance for red, green, and blue; `Tolerance(n)` applies the
same variation to all channels. `RGB.Matches` is inclusive on every channel.

`Bitmap` is a tightly packed RGBA image with public `Width`, `Height`, and
`Pixels` fields. It implements `image.Image`, supports `RGBAt`, `Set`,
`Clone`, `Crop`, `MatchesAll`, and PNG output through `WritePNG`/`SavePNG`.
The fourth byte is storage padding: bitmap image operations and PNG output are
always opaque, matching screen-capture semantics. `WritePNG` uses the backing
pixels directly when their alpha bytes are already 255 and otherwise
normalizes a private copy without mutating the bitmap.
Bitmap and capture dimensions are overflow-checked and capped at 512 MiB to
avoid malformed input or unreasonable allocations. `RGBAt` and `Set` quietly
ignore out-of-range coordinates; constructors and operations that must report
bad geometry return an error.

`RelativePoint` and `Window.ScalePoint` scale a reference client-coordinate
point to a current client size using integer arithmetic. This is useful for
resolution-relative coordinates in fixed-layout applications, but does not
replace testing an actual responsive layout.

Use `Window.ClientToScreen`, `Window.ScreenToClient`, and `Window.ClientOrigin`
only when a boundary needs to be crossed. Window pixel/capture/search helpers
already interpret locations as client coordinates, matching `CoordMode, Pixel,
Relative`. Without an appropriate DPI awareness context, Windows may virtualize
coordinates on high-DPI displays; call `SetDPIAware` early and use `WindowDPI`
when the effective DPI is needed.

## Capture and pixel reads

```go
// Physical screen coordinates.
screen, err := automation.CaptureScreen(automation.InclusiveRect(0, 0, 799, 599))
color, err := automation.PixelColor(100, 100)

// Client-area coordinates relative to the target window.
region, err := target.CaptureRegion(
	automation.Rect{Left: 120, Top: 80, Right: 520, Bottom: 320},
	automation.CaptureOptions{},
)
color, err = target.Pixel(100, 100)
```

Capture produces an owned RGBA `Bitmap`. Internally, Windows capture uses a
top-down 32-bit DIB section, copies with GDI, converts native BGRA into the
public RGBA layout, and releases every DC/bitmap selection on every return
path. Native dimensions and Go allocation sizes are checked before allocation.

`CaptureOptions` selects the following behavior:

| Method | Behavior | Use it when | Limitation |
| --- | --- | --- | --- |
| `CaptureVisible` (zero/default) | Copies the visible desktop pixels beneath the requested screen/client rectangle. | Matching what a person can currently see; normal pixel/image searches. | Other windows, overlays, minimization, or off-screen placement are reflected in the bitmap. |
| `CapturePrintWindow` | Calls `PrintWindow` synchronously into a local DIB. `ClientOnly` requests client rendering. | A conventional desktop window is obscured but supports `PrintWindow`. | It can block in the target window procedure and commonly returns blank/stale content for DirectX, Vulkan, or other GPU surfaces. |
| `CaptureAuto` | Tries `PrintWindow`, then uses visible capture only if the API call reports failure. | Best-effort desktop-window capture. | A technically successful but all-black/stale `PrintWindow` image cannot be reliably detected and is returned. |

`CaptureWindow` captures the full client area when `ClientOnly` is true or the
full window rectangle otherwise. `CaptureWindowRegion` is always
client-relative, clips its requested region to the client area, and rejects an
empty result. Visible capture reads only that region; `PrintWindow` must render
the full client area first and the library then crops it.

`PixelColorWindow(hwnd, x, y, client)` converts `x,y` from client coordinates
when `client` is true; `Window.Pixel` is the convenient client-relative form.
`CursorPosition` only reads the cursor position and never moves it.

## Pixel and image search

### Pixel search

`SearchPixel` matches AHK `PixelSearch` scan semantics: both endpoints are
included, and reversing either endpoint reverses that axis' scan direction.
Coordinates outside the bitmap are clipped. `SearchPixelRect` uses a half-open
rectangle and always scans top-to-bottom, left-to-right after normalization.
Both return the first match and `false` when no match exists.

```go
point, found, err := target.SearchPixel(
	automation.InclusiveRect(1450, 325, 1520, 349),
	automation.RGB{R: 92, G: 225, B: 0},
	automation.ColorTolerance{R: 30, G: 30, B: 2},
)
```

The returned point is client-relative to `target`, not relative to the supplied
region. A missing match is `(Point{}, false, nil)`.

### Image search

`ImageSearchOptions` covers the useful AHK `ImageSearch` options:

| Field/option | Meaning |
| --- | --- |
| `Variation` / `*n` | Maximum independent difference per RGB channel, from 0 through 255. |
| `Transparent` / `*Trans...` | Template pixels with that exact RGB value are wildcards. Hex `RRGGBB`, `0xRRGGBB`, `#RRGGBB`, and common AHK/VGA names are accepted. |
| `Width`, `Height` / `*w`, `*h` | Optional output template dimensions. `-1` preserves aspect ratio; zero retains the source dimension. |

Accepted color names are `Black`, `White`, `Red`, `Green`, `Blue`, `Yellow`,
`Magenta`/`Fuchsia`, `Cyan`/`Aqua`, `Gray`/`Grey`, `Silver`, `Maroon`,
`Purple`, `Lime`, `Olive`, `Navy`, `Teal`, and `Orange`.

`ParseImageSearchOptions` accepts an AHK-style string such as
`*42 *TransBlack *w100 *h-1 "C:\\Program Files\\icon.png"` and returns the
options and unquoted path. It rejects unknown options rather than silently
changing matching behavior.

`LoadImage` and `LoadTemplate` decode formats registered with Go's `image`
package, including PNG, JPEG, GIF, BMP, and TIFF. Both preflight the encoded
dimensions before full decoding, reject decoded storage above 256 MiB, and
revalidate the decoded bounds. `SearchImage` is the convenient one-off API; it
compiles every call. For polling, use `CompileTemplate` or `LoadTemplate` once,
then call `SearchTemplate`:

```go
transparent := automation.RGB{R: 255, G: 0, B: 255}
tpl, err := automation.LoadTemplate("img/target.png", automation.ImageSearchOptions{
	Transparent: &transparent,
})
if err != nil {
	return err
}

point, found, err := automation.SearchTemplate(bitmap, automation.Rect{}, tpl)
```

Template compilation performs decoding/scaling once, folds transparent colors
and alpha-zero source pixels into a wildcard mask, chooses a small set of
opaque anchor pixels for early rejection, and makes a private copy of option
state. A `Template` is immutable and safe to share among concurrent searches.
Search returns the first top-left match in row-major order. Template
allocations are also overflow-checked and capped at 256 MiB.

The hot `SearchTemplate` path has no allocations: it scans source bytes
directly, checks anchors first, then checks all opaque template pixels. Its
cost is still proportional to the candidate region and template complexity.
Keep the region tight, reuse compiled templates, and choose a distinctive
template with few wildcards for the best result.

## Windows and application control

`Window` is a lightweight wrapper around an unowned `HWND`. A handle can become
invalid at any time because the target window belongs to another process.
Operations validate it when Windows provides a meaningful validation point and
otherwise return a Windows error or `ErrNotFound`.

`WindowQuery` combines all non-zero/non-empty filters with AND semantics:

- `Title`: exact, case-insensitive title.
- `TitleContains`: case-insensitive title substring.
- `Class`: exact, case-insensitive window class.
- `Process`: case-insensitive full process path or basename, such as
  `example-app.exe`. This opens a query-limited process handle while filtering.
- `PID`: exact process identifier.
- `VisibleOnly`: excludes windows Windows reports as not visible.

`FindWindows` returns all matches in Windows enumeration order. `FindWindow`
stops at and returns the first match; no match is `ErrNotFound`. `ActiveWindow`
returns the foreground window. `Window` also exposes title, class, PID, process
path, window/client rectangles, client origin, visibility/minimized/maximized
state, and coordinate conversion.

State operations are direct wrappers over normal window messages/APIs:

| Goal | API | Notes |
| --- | --- | --- |
| Focus | `Window.Activate` | Restores a minimized window, brings it to top, and requests foreground activation. Windows focus-stealing rules can deny the request. |
| Show, hide, minimize, maximize, restore, close | Corresponding `Window` method | `Close` posts `WM_CLOSE`; it requests a normal close and does not force termination. |
| Toggle | `Window.Toggle` or `ToggleWindow` | A minimized window is activated; an active window is minimized; every other existing window is activated. |
| Move/resize | `SetBounds`, `Move`, `Resize` | Bounds are screen/window-rectangle bounds, not client bounds. Invalid or overflowing geometry is rejected. |
| Place without resizing | `Center`, `Snap` | `SnapCenter`, `SnapLeft`, `SnapRight`, `SnapTop`, and `SnapBottom` position the current window rectangle within supplied bounds. |

`App` groups a `WindowQuery` with an optional `Command` and `Args`.
`App.Toggle` only toggles an existing matching window. `App.ToggleOrStart`
toggles one when found; otherwise it starts `Command` and returns the started
`*exec.Cmd` with an empty `Window`. It does not wait for the new process to
create a window. It copies `Args` before passing them to `os/exec`, so later
caller mutation cannot alter launch arguments.

## Displays and process priority

`ScreenRect` returns the virtual desktop rectangle, including negative
coordinates for monitors placed left of or above the primary display.
`PrimaryScreenRect` returns the primary display. `Monitors` reports each
monitor's full rectangle, work area, and primary flag.

`CurrentDisplayMode`, `DisplayModes`, `SetDisplayMode`, and
`RestoreDisplayMode` operate on the primary display through documented display
settings APIs. `SetDisplayMode` validates dimensions and optional
bits-per-pixel/frequency values before the call. Its `permanent` argument
requests registry persistence; a temporary mode lasts for the session.
`DisplayModes` deduplicates driver results and bounds enumeration at 4096
entries, returning an error instead of looping forever or returning a silently
truncated list if a driver never reports completion.
Changing display mode is a system-wide side effect and must be an explicit
caller decision. `RestoreDisplayMode` requests the saved/default configuration.

`SetProcessPriority(pid, priority)` and `Window.SetProcessPriority` expose only
the standard Windows idle, below-normal, normal, above-normal, and high
priority classes. They do not offer realtime priority. Permission errors are
returned to the caller.

## Timing and polling

| API | Design and use |
| --- | --- |
| `Sleep(ctx, d)` | Context-aware sleep. A non-positive duration returns after checking cancellation. |
| `PreciseSleep(ctx, d)` | Sleeps for most of the duration and yields in a final roughly 2 ms window to reduce scheduler overshoot. This uses bounded CPU near the deadline and does not alter system timer resolution. |
| `JitterSleep(ctx, d, minus, plus, rng)` | Sleeps for a uniformly selected duration in `[max(0,d-minus), max(0,d+plus)]`. Negative jitter values are treated as zero and positive overflow is rejected. Jitter is caller-directed scheduling behavior, not a claim of invisibility. |
| `WaitUntil(ctx, interval, predicate)` | Evaluates immediately, then reuses one ticker between evaluations. It never busy-waits, stops promptly on cancellation, and propagates predicate errors. |
| `BeginTimerResolution(period)` | Acquires a WinMM timer-resolution lease (zero requests the 1 ms default). Always `defer lease.Close()`; close is idempotent. |

Nil contexts and predicates are rejected with `ErrInvalidArgument` instead of
panicking. Use a timer-resolution lease only when measurement shows it is
necessary because it can affect timer granularity and power use.

## Keyboard and mouse polling

`Key` is a Windows virtual-key code. The package exports constants for the
standard keyboard, modifier, function, navigation, media, OEM, and mouse keys.
`ParseKey` accepts those names case-insensitively, single ASCII letters and
digits, `F1` through `F24`, `Numpad0` through `Numpad9`, and `VK_XX` hex codes.

`IsKeyDown` reads Windows' current asynchronous state with the high-order bit of
`GetAsyncKeyState`; it does not use or consume the API's low-order transition
bit. `KeyToggleOn` reads the lock-key toggle bit with `GetKeyState`, such as
`CapsLock`, `NumLock`, or `ScrollLock`. A generic logical-state API is omitted:
that state belongs to an OS thread's input queue, while Go goroutines can move
between OS threads.

Windows can also return zero when this process cannot query the active desktop,
and the API provides no separate failure result. Polling reports system state;
it does not establish whether a down key originated from hardware, synthesized
input, or another allowed input source.

For a trigger loop, pass the keys of interest to `PollInput` and retain the
returned value as the next iteration's baseline:

```go
keys := []automation.Key{automation.KeyF8, automation.KeyCtrl, automation.KeyLButton}
previous, err := automation.PollInput(keys...)
if err != nil {
	return err
}
for {
	current, err := automation.PollInput(keys...)
	if err != nil {
		return err
	}
	if current.PressedSince(previous, automation.KeyF8) &&
		current.AllDown(automation.KeyCtrl) {
		// Trigger caller-owned work here; no input is sent by automation.
	}
	previous = current
	if err := automation.Sleep(ctx, 16*time.Millisecond); err != nil {
		return err
	}
}
```

`InputSnapshot` is an immutable value. `Sampled`, `Down`, `AnyDown`,
`AllDown`, `PressedSince`, and `ReleasedSince` make key and chord checks
explicit. Both snapshots must contain a key for an edge to be reported, which
prevents the first sample from looking like a press. Duplicate keys in one
poll are read once. The native calls are sequential rather than atomic, so a
key changing during a multi-key sample may be observed in the next iteration.
Polling can miss a press and release that both happen between samples; use a
caller-chosen interval appropriate for the trigger.

`MouseButtonKey` and `MouseButtonDown` expose logical primary, secondary,
middle, X1, and X2 mouse buttons. Primary/secondary resolution follows the
current Windows swapped-button setting. `CursorPosition` remains a separate
read-only coordinate query. No input hooks, hidden goroutines, or process-wide
event queues are installed.

Mouse-wheel movement is not a down/up state and therefore cannot be observed
by this polling API. Capturing wheel events would require a message target,
raw-input registration, or a hook, none of which `automation` installs.

## Concurrency and performance contracts

| Component | Contract |
| --- | --- |
| `Template` | Immutable after construction and safe for concurrent `SearchTemplate` calls. |
| `Bitmap` | Concurrent reads are safe after publication only while no goroutine mutates `Pixels` or calls `Set`. The type deliberately has no lock because it is a data container. |
| `TimerResolution.Close` | Idempotent and concurrency-safe. |
| `JitterSleep` with a supplied `*rand.Rand` | The call serializes its use of that generator. Code that also uses the generator directly must synchronize its own access. |
| DPI initialization | `SetDPIAware` is process-wide, initialized once, and safe for concurrent callers. |
| `InputSnapshot` / input reads | Snapshots are immutable values; `IsKeyDown`, `KeyToggleOn`, `PollInput`, and snapshot reads have no shared mutable package state and are safe for concurrent callers. |
| Windows handles and screen state | No operation can make a target window, desktop composition, or display layout stable. Handle a window disappearing, moving, being covered, or changing between calls. |

The design favors short-lived native resources, bounded allocations, and no
package-owned background goroutines. Every capture returns a new bitmap;
templates are the reusable performance object. For high-rate polling, compile
templates once, capture the smallest client region containing the target,
choose a reasonable `WaitUntil` interval, and acquire timer resolution only
when it is justified by measurement.

## Errors and search results

- `ErrNotFound`: no matching window/resource, an invalid handle, or an absent
  required target.
- `ErrInvalidRect`: empty, impossible, overflowing, or allocation-exceeding
  geometry.
- `ErrInvalidArgument`: invalid enum, nil required callback/context, malformed
  bitmap/template storage, or invalid option/input value.
- `ErrUnsupported`: a Windows-only operation invoked on a non-Windows build.

Windows failures generally add operation context and preserve a non-zero
Win32 error when available. Search misses are not errors: search methods return
`found == false` with `err == nil`.

## AHK mapping

| AHK functionality | Go API |
| --- | --- |
| `PixelGetColor`, RGB checks | `PixelColor`, `Window.Pixel`, `RGB.Matches`, `Bitmap.MatchesAll` |
| `PixelSearch` | `SearchPixel`, `SearchPixelRect`, `Window.SearchPixel` |
| `ImageSearch *n *Trans... *w... *h...` | `LoadTemplate`, `CompileTemplate`, `SearchTemplate`, `Window.SearchTemplate`, `ParseImageSearchOptions` |
| `CoordMode, Pixel, Relative` | `Window.CaptureRegion`, `Window.Pixel`, `ClientToScreen`, `ScreenToClient` |
| Resolution-relative points | `RelativePoint`, `Window.ScalePoint` |
| Screen/window image capture | `CaptureScreen`, `CaptureWindow`, `CaptureWindowRegion`, `Bitmap.SavePNG` |
| `WinActive`, `WinExist`, `WinGetPos` | `ActiveWindow`, `FindWindow(s)`, `Window.Rect`, `Window.ClientRect` |
| `WinActivate`, minimize/restore, app toggle | `Window.Activate`, state methods, `Window.Toggle`, `App.Toggle` |
| `WinMove`, center/edge placement | `SetBounds`, `Move`, `Resize`, `Center`, `Snap` |
| `Run` when absent | `App.ToggleOrStart` |
| `A_ScreenWidth`, monitor bounds | `PrimaryScreenRect`, `ScreenRect`, `Monitors` |
| `EnumDisplaySettings`, `ChangeDisplaySettings` | `CurrentDisplayMode`, `DisplayModes`, `SetDisplayMode`, `RestoreDisplayMode` |
| `Sleep`, randomized sleep, `timeBeginPeriod` | `Sleep`, `PreciseSleep`, `JitterSleep`, `BeginTimerResolution` |
| `Process, Priority` | `SetProcessPriority`, `Window.SetProcessPriority` |
| Read-only `MouseGetPos` | `CursorPosition` |
| `GetKeyState` physical/toggle checks | `IsKeyDown`, `KeyToggleOn` |
| Key/button trigger checks | `PollInput`, `InputSnapshot`, `MouseButtonDown` |

Keyboard/mouse sending, hotkey registration, HID emulation, cursor clipping,
application-specific decision sequences, and presentation/debug UI remain
outside this package. Read-only polling does not create an event stream and
does not guarantee that very short transitions are observed.

## Verification and references

The package has unit tests for geometry, pixel and image semantics, image
options, ownership and allocation guards, timing, and concurrent use. Windows
integration tests exercise the native contracts.

```powershell
go vet ./automation
go test ./automation
go test -race ./automation
go test -run TestWindows -count=1 ./automation
```

Semantics were checked against the [AutoHotkey v2 ImageSearch documentation](https://www.autohotkey.com/docs/v2/lib/ImageSearch.htm), [AutoHotkey v2 PixelSearch documentation](https://www.autohotkey.com/docs/v2/lib/PixelSearch.htm), [AutoHotkey v2 GetKeyState documentation](https://www.autohotkey.com/docs/v2/lib/GetKeyState.htm), [Microsoft GetAsyncKeyState documentation](https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-getasynckeystate), [Microsoft GetKeyState documentation](https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-getkeystate), [Microsoft BitBlt documentation](https://learn.microsoft.com/windows/win32/api/wingdi/nf-wingdi-bitblt), and [Microsoft PrintWindow documentation](https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-printwindow).
