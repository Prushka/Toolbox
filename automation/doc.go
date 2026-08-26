// Package automation contains low-level, observation-oriented Windows
// automation primitives. It deliberately does not synthesize keyboard or
// mouse input, inject into processes, or bypass application security
// boundaries. InputMonitor provides event-driven keyboard hotkeys through
// RegisterHotKey and opt-in pass-through mouse-button monitoring through a
// low-level mouse hook. IsKeyDown and PollInput remain available for direct
// state queries.
//
// Capture and power-request functions use documented Windows APIs.
// Window-relative helpers
// treat coordinates as client-area coordinates, matching AutoHotkey's
// CoordMode Pixel Relative behavior. The package does not claim invisibility;
// its low footprint comes from documented APIs, a single message thread,
// installing the mouse hook only while needed, and avoiding keyboard hooks,
// injection, process memory access, and background polling loops.
package automation
