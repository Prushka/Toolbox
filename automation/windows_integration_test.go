//go:build windows

package automation

import (
	"errors"
	"os"
	"sync"
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

func TestWindowsDIBRegionConversion(t *testing.T) {
	const sourceWidth, sourceHeight = 4, 3
	native := []byte{
		1, 2, 3, 0, 4, 5, 6, 0, 7, 8, 9, 0, 10, 11, 12, 0,
		13, 14, 15, 0, 16, 17, 18, 0, 19, 20, 21, 0, 22, 23, 24, 0,
		25, 26, 27, 0, 28, 29, 30, 0, 31, 32, 33, 0, 34, 35, 36, 0,
	}
	b, err := dibRegionToBitmap(unsafe.Pointer(&native[0]), sourceWidth, sourceHeight, Rect{1, 1, 3, 3})
	if err != nil {
		t.Fatal(err)
	}
	if b.Width != 2 || b.Height != 2 {
		t.Fatalf("region bitmap=%dx%d", b.Width, b.Height)
	}
	want := []RGB{{18, 17, 16}, {21, 20, 19}, {30, 29, 28}, {33, 32, 31}}
	for i, color := range want {
		if got := b.RGBAt(i%2, i/2); got != color {
			t.Errorf("pixel %d=%v, want %v", i, got, color)
		}
	}
}

func TestWindowsDIBRegionConversionRejectsInvalidGeometry(t *testing.T) {
	native := []byte{1, 2, 3, 0}
	tests := []struct {
		name   string
		bits   unsafe.Pointer
		width  int
		height int
		region Rect
		want   error
	}{
		{name: "nil bits", width: 1, height: 1, region: Rect{0, 0, 1, 1}, want: ErrInvalidArgument},
		{name: "zero source width", bits: unsafe.Pointer(&native[0]), height: 1, region: Rect{0, 0, 1, 1}, want: ErrInvalidRect},
		{name: "empty region", bits: unsafe.Pointer(&native[0]), width: 1, height: 1, region: Rect{}, want: ErrInvalidRect},
		{name: "reversed region", bits: unsafe.Pointer(&native[0]), width: 1, height: 1, region: Rect{1, 1, 0, 0}, want: ErrInvalidRect},
		{name: "negative origin", bits: unsafe.Pointer(&native[0]), width: 1, height: 1, region: Rect{-1, 0, 1, 1}, want: ErrInvalidRect},
		{name: "outside source", bits: unsafe.Pointer(&native[0]), width: 1, height: 1, region: Rect{0, 0, 2, 1}, want: ErrInvalidRect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := dibRegionToBitmap(tt.bits, tt.width, tt.height, tt.region); !errors.Is(err, tt.want) {
				t.Fatalf("dibRegionToBitmap error=%v, want %v", err, tt.want)
			}
		})
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

func TestWindowsRejectsInvalidArguments(t *testing.T) {
	if _, err := CaptureWindow(HWND(1), CaptureOptions{Method: CaptureMethod(255)}); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid capture method=%v", err)
	}
	if unsafe.Sizeof(int(0)) > 4 {
		if _, err := CaptureScreen(Rect{maxInt - 1, 0, maxInt, 1}); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("invalid screen rectangle=%v", err)
		}
	}
	if _, _, _, err := makeDIB(1<<30, 1); !errors.Is(err, ErrInvalidRect) {
		t.Fatalf("oversized DIB=%v", err)
	}
	if err := SetDisplayMode(DisplayMode{Width: 1, Height: 1, Frequency: -1}, false); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid display mode=%v", err)
	}
	if err := SetProcessPriority(uint32(os.Getpid()), ProcessPriority(1)); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid process priority=%v", err)
	}
	if _, err := IsKeyDown(0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid key state=%v", err)
	}
	if _, err := KeyToggleOn(0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid toggle key=%v", err)
	}
	if _, err := PollInput(); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("empty input poll=%v", err)
	}
	if _, err := MouseButtonKey(0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("invalid mouse button=%v", err)
	}
}

func TestWindowsInputPolling(t *testing.T) {
	if _, err := IsKeyDown(KeyF24); err != nil {
		t.Fatal(err)
	}
	if _, err := KeyToggleOn(KeyCapsLock); err != nil {
		t.Fatal(err)
	}
	snapshot, err := PollInput(KeyLShift, KeyRShift, KeyLButton, KeyLShift)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []Key{KeyLShift, KeyRShift, KeyLButton} {
		if !snapshot.Sampled(key) {
			t.Fatalf("key %s was not sampled", key)
		}
	}
	primary, err := MouseButtonKey(MousePrimary)
	if err != nil {
		t.Fatal(err)
	}
	secondary, err := MouseButtonKey(MouseSecondary)
	if err != nil {
		t.Fatal(err)
	}
	validPrimary := primary == KeyLButton || primary == KeyRButton
	validSecondary := secondary == KeyLButton || secondary == KeyRButton
	if primary == secondary || !validPrimary || !validSecondary {
		t.Fatalf("logical mouse mapping primary=%s secondary=%s", primary, secondary)
	}
	for _, button := range []MouseButton{MousePrimary, MouseSecondary, MouseMiddle, MouseX1, MouseX2} {
		key, err := MouseButtonKey(button)
		if err != nil || !key.Valid() {
			t.Fatalf("button %d resolved to %v, %v", button, key, err)
		}
		if _, err := MouseButtonDown(button); err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkPollInput(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = PollInput(KeyCtrl, KeyShift, KeyF8, KeyLButton)
	}
}

func TestSetDPIAwareConcurrent(t *testing.T) {
	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- SetDPIAware()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWindowsEnumerationConcurrent(t *testing.T) {
	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < 8; iteration++ {
				if worker%2 == 0 {
					windows, err := FindWindows(WindowQuery{})
					if err != nil {
						errs <- err
						return
					}
					if len(windows) == 0 {
						errs <- errors.New("no top-level windows")
						return
					}
					continue
				}
				monitors, err := Monitors()
				if err != nil {
					errs <- err
					return
				}
				if len(monitors) == 0 {
					errs <- errors.New("no monitors")
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
