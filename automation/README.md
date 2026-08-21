# automation

`automation` is a low-level Go package for the observation and window-management parts of the AHK scripts in `btd6-ahk` and `GenshinAHK`. It does not generate input.

The default capture path uses documented GDI screen capture and client/screen coordinate translation. It does not inject code, install hooks, read game memory, load a DLL into another process, or open the target process. This keeps the design small and passive, but it is not a promise of invisibility. `CapturePrintWindow` is an explicit alternative for obscured windows; it sends a synchronous render request to the target and often does not work for GPU-rendered games.

Call `SetDPIAware` at process startup, before creating any windows, when physical-pixel accuracy is required.

## Quick start

```go
import (
    "context"
    "time"
    "github.com/Prushka/Toolbox/automation"
)

func observe(ctx context.Context) error {
_ = automation.SetDPIAware()

game, err := automation.FindWindow(automation.WindowQuery{
	Process:     "BloonsTD6.exe",
	VisibleOnly: true,
})
if err != nil {
	return err
}

green := automation.RGB{R: 92, G: 225, B: 0}
found, ok, err := game.SearchPixel(
	automation.InclusiveRect(1450, 325, 1520, 349),
	green,
	automation.ColorTolerance{R: 30, G: 30, B: 2},
)
if err != nil { return err }
_ = found
_ = ok

tpl, err := automation.LoadTemplate(`img\states\victory.png`, automation.ImageSearchOptions{
	Variation: 42,
})
if err != nil { return err }
err = automation.WaitUntil(ctx, 16*time.Millisecond, func() (bool, error) {
	_, found, err := game.SearchTemplate(automation.Rect{}, tpl)
	return found, err
})
return err
}
```

Compile templates once with `LoadTemplate` or `CompileTemplate` when searching repeatedly. Capture only the region needed; `Window.SearchPixel`, `Window.SearchTemplate`, and `Window.CaptureRegion` do this automatically.

## AHK mapping

| AHK usage in the source projects | Go API |
| --- | --- |
| `PixelGetColor` / RGB tolerance helpers | `PixelColor`, `Window.Pixel`, `RGB.Matches`, `Bitmap.MatchesAll` |
| `PixelSearch` | `SearchPixel`, `SearchPixelRect`, `Window.SearchPixel` |
| `ImageSearch *n *Trans... *w... *h...` | `LoadTemplate`, `SearchTemplate`, `Window.SearchTemplate`, `ParseImageSearchOptions` |
| `CoordMode, Pixel, Relative` | `Window.CaptureRegion`, `ClientToScreen`, `ScreenToClient` |
| Resolution-relative coordinates | `RelativePoint`, `Window.ScalePoint` |
| Screen/image capture | `CaptureScreen`, `CaptureWindow`, `CaptureWindowRegion`, `Bitmap.SavePNG` |
| `WinActive`, `WinExist`, `WinGetPos` | `ActiveWindow`, `FindWindow(s)`, `Window.Rect`, `Window.ClientRect` |
| `WinActivate`, minimize/restore, app toggle | `Window.Activate`, state methods, `Window.Toggle`, `App.Toggle` |
| `WinMove` / center or edge placement | `Window.Move`, `SetBounds`, `Resize`, `Center`, `Snap` |
| `Run` when an app is absent | `App.ToggleOrStart` |
| `A_ScreenWidth/Height`, monitor bounds | `PrimaryScreenRect`, `ScreenRect`, `Monitors` |
| `EnumDisplaySettings` / `ChangeDisplaySettings` | `CurrentDisplayMode`, `DisplayModes`, `SetDisplayMode`, `RestoreDisplayMode` |
| `Sleep`, randomized sleep, `timeBeginPeriod` | `Sleep`, `PreciseSleep`, `JitterSleep`, `BeginTimerResolution` |
| `Process, Priority` | `SetProcessPriority`, `Window.SetProcessPriority` |
| `IniRead` / `IniWrite` | `ReadINI`, `WriteINI`, `INI` |
| timestamped `FileAppend` logging | `Logger` |
| read-only `MouseGetPos` | `CursorPosition` |

AHK keyboard/mouse state, hotkeys, click/move/press/hold, cursor clipping, serial/HID emulation, and game-specific decision sequences are intentionally absent. The source projects' debug GUIs and crosshair are presentation layers rather than low-level automation primitives.

## Capture behavior

- `CaptureVisible` is the default and captures what is actually visible, like AHK pixel and image search. Occlusion is therefore visible in the result.
- `CapturePrintWindow` asks the target to render through `PrintWindow`; it can capture some obscured desktop windows but may block and often returns blank content for DirectX/Vulkan windows.
- `CaptureAuto` tries `PrintWindow` and falls back to visible capture only when the API reports failure. It cannot reliably identify an all-black but technically successful result.
- Rectangles are half-open. Use `InclusiveRect` when porting AHK's inclusive `X1,Y1,X2,Y2` regions.

Matching follows the AutoHotkey v2 documentation: channel variation is independent, transparent template pixels and alpha-zero pixels are wildcards, and the first match is returned in row-major order. `SearchPixel` also preserves AHK's reverse scan order when endpoints are reversed.

Reference semantics were checked against the [AutoHotkey ImageSearch documentation](https://www.autohotkey.com/docs/v2/lib/ImageSearch.htm), [AutoHotkey PixelSearch documentation](https://www.autohotkey.com/docs/v2/lib/PixelSearch.htm), [Microsoft BitBlt documentation](https://learn.microsoft.com/windows/win32/api/wingdi/nf-wingdi-bitblt), and [Microsoft PrintWindow documentation](https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-printwindow).
