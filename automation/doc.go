// Package automation contains low-level, observation-oriented Windows
// automation primitives. It deliberately does not synthesize keyboard or
// mouse input, install hooks, inject into processes, or bypass application
// security boundaries.
//
// Capture functions use documented GDI/User32 APIs. Window-relative helpers
// treat coordinates as client-area coordinates, matching AutoHotkey's
// CoordMode Pixel Relative behavior.
package automation
