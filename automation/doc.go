// Package automation contains low-level, observation-oriented Windows
// automation primitives. It deliberately does not synthesize keyboard or
// mouse input, install hooks, inject into processes, or bypass application
// security boundaries. Read-only keyboard and mouse polling is available
// through IsKeyDown and PollInput; callers own the polling loop and actions.
//
// Capture functions use documented GDI/User32 APIs. Window-relative helpers
// treat coordinates as client-area coordinates, matching AutoHotkey's
// CoordMode Pixel Relative behavior. The package does not claim invisibility;
// its low footprint comes from passive, documented APIs and avoiding hooks,
// injection, process memory access, and background polling goroutines.
package automation
