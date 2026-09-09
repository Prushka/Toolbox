//go:build windows

package automation

import (
	"fmt"
	"runtime"
	"testing"
	"unsafe"
)

func TestWindowsCapturePendingGDIDrawing(t *testing.T) {
	patBlt := gdi32.NewProc("PatBlt")
	for worker := range 8 {
		t.Run(fmt.Sprint(worker), func(t *testing.T) {
			t.Parallel()
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			dc, _, cleanup, err := makeDIB(16, 16)
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			for iteration := range 40 {
				operation, want := uintptr(0x00000042), RGB{}
				if iteration%2 == 0 {
					operation, want = 0x00FF0062, RGB{255, 255, 255}
				}
				if ret, _, err := patBlt.Call(dc, 0, 0, 16, 16, operation); ret == 0 {
					t.Fatal(winCallError(err, "PatBlt failed"))
				}
				// Boolean GDI drawing calls can still be queued. Capture must see
				// this iteration's completed drawing before it reads DIB memory.
				bitmap, err := captureFromDC(dc, 0, 0, 16, 16)
				if err != nil {
					t.Fatal(err)
				}
				for y := range 16 {
					for x := range 16 {
						if got := bitmap.RGBAt(x, y); got != want {
							t.Fatalf("iteration %d pixel %d,%d = %v, want %v", iteration, x, y, got, want)
						}
					}
				}
			}
		})
	}
}

func TestWindowsPowerRequestContextABI(t *testing.T) {
	wantSize := uintptr(32)
	if unsafe.Sizeof(uintptr(0)) == 4 {
		wantSize = 24
	}
	context := powerRequestContext{}
	if got := unsafe.Sizeof(context); got != wantSize {
		t.Fatalf("REASON_CONTEXT size = %d, want %d", got, wantSize)
	}
	if got := unsafe.Offsetof(context.SimpleReasonString); got != 8 {
		t.Fatalf("REASON_CONTEXT union offset = %d, want 8", got)
	}
}
