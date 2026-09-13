package automation

import (
	"errors"
	"testing"
)

func TestDisplayRequiredCaptureRejectsLossBeforeOrDuringCapture(t *testing.T) {
	for _, tc := range []struct {
		name         string
		required     bool
		loseAt       int
		captureErr   error
		wantCaptures int
		want         error
	}{
		{"ordinary capture unchanged", false, 1, nil, 1, nil},
		{"connected", true, 0, nil, 1, nil},
		{"disconnected before capture", true, 1, nil, 0, ErrDisplayUnavailable},
		{"disconnected during capture", true, 2, nil, 1, ErrDisplayUnavailable},
		{"capture failure retained with disconnect", true, 2, ErrInvalidRect, 1, ErrDisplayUnavailable},
		{"capture failure retained with display", true, 0, ErrInvalidRect, 1, ErrInvalidRect},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks, captures := 0, 0
			frame := &Bitmap{}
			got, err := captureWithDisplayCheck(tc.required, func() error {
				checks++
				if checks == tc.loseAt {
					return ErrDisplayUnavailable
				}
				return nil
			}, func() (*Bitmap, error) { captures++; return frame, tc.captureErr })
			if !errors.Is(err, tc.want) || captures != tc.wantCaptures {
				t.Fatalf("error=%v captures=%d", err, captures)
			}
			if tc.captureErr != nil && !errors.Is(err, tc.captureErr) {
				t.Fatalf("lost capture error: %v", err)
			}
			if errors.Is(err, ErrDisplayUnavailable) && got != nil {
				t.Fatal("disconnected capture returned a bitmap")
			}
			if !tc.required && checks != 0 {
				t.Fatal("ordinary capture queried topology")
			}
		})
	}
}
