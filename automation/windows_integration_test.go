//go:build windows

package automation

import (
	"testing"
	"time"
	"unsafe"
)

func TestWindowsCaptureAndDisplayQueries(t *testing.T) {
	_ = SetDPIAware()
	if got := unsafe.Sizeof(devMode{}); got != 220 {
		t.Fatalf("DEVMODEW size=%d, want 220", got)
	}
	s := PrimaryScreenRect()
	if s.Empty() {
		t.Fatalf("primary screen=%+v", s)
	}
	r := Rect{s.Left, s.Top, s.Left + 16, s.Top + 16}
	b, e := CaptureScreen(r)
	if e != nil {
		t.Fatal(e)
	}
	if b.Width != 16 || b.Height != 16 || len(b.Pixels) != 16*16*4 {
		t.Fatalf("capture=%dx%d/%d", b.Width, b.Height, len(b.Pixels))
	}
	c, e := PixelColor(s.Left, s.Top)
	if e != nil {
		t.Fatal(e)
	}
	one, e := CaptureScreen(Rect{s.Left, s.Top, s.Left + 1, s.Top + 1})
	if e != nil {
		t.Fatal(e)
	}
	if c != one.RGBAt(0, 0) {
		t.Logf("screen changed between point and frame capture: %v != %v", c, one.RGBAt(0, 0))
	}
	mode, e := CurrentDisplayMode()
	if e != nil {
		t.Fatal(e)
	}
	if mode.Width <= 0 || mode.Height <= 0 || mode.BitsPerPixel <= 0 {
		t.Fatalf("mode=%+v", mode)
	}
	modes, e := DisplayModes()
	if e != nil || len(modes) == 0 {
		t.Fatalf("modes=%d,%v", len(modes), e)
	}
	monitors, e := Monitors()
	if e != nil || len(monitors) == 0 {
		t.Fatalf("monitors=%d,%v", len(monitors), e)
	}
}

func TestWindowsAndWindowQueries(t *testing.T) {
	ws, e := FindWindows(WindowQuery{})
	if e != nil {
		t.Fatal(e)
	}
	if len(ws) == 0 {
		t.Fatal("no top-level windows")
	}
	if !ws[0].Valid() {
		t.Fatal("enumerated invalid window")
	}
	w, e := ActiveWindow()
	if e != nil {
		t.Skipf("no active interactive window: %v", e)
	}
	if !w.Valid() {
		t.Fatal("active window invalid")
	}
	client, e := w.ClientRect()
	if e == nil && client.Width() >= 8 && client.Height() >= 8 {
		shot, e := w.CaptureRegion(Rect{0, 0, 8, 8}, CaptureOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if shot.Width != 8 || shot.Height != 8 {
			t.Fatalf("client capture=%dx%d", shot.Width, shot.Height)
		}
	}
	p := Point{7, 9}
	screen, e := w.ClientToScreen(p)
	if e != nil {
		t.Fatal(e)
	}
	back, e := w.ScreenToClient(screen)
	if e != nil {
		t.Fatal(e)
	}
	if absInt(back.X-p.X) > 1 || absInt(back.Y-p.Y) > 1 {
		t.Fatalf("round trip %v -> %v -> %v", p, screen, back)
	}
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestTimerResolutionLifecycle(t *testing.T) {
	r, e := BeginTimerResolution(1)
	if e != nil {
		t.Fatal(e)
	}
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	if e := PreciseSleep(t.Context(), 2*time.Millisecond); e != nil {
		t.Fatal(e)
	}
	if time.Since(start) < 1500*time.Microsecond {
		t.Fatal("precise sleep returned early")
	}
}
