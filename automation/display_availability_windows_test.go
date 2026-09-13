//go:build windows

package automation

import (
	"encoding/binary"
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestRequireActiveDisplayUsesAvailableActiveTargets(t *testing.T) {
	for _, tc := range []struct {
		name     string
		paths    [][2]uint32 // targetAvailable, path flags
		queryErr error
		want     error
	}{
		{name: "no paths", want: ErrDisplayUnavailable},
		{name: "fallback without active paths", queryErr: ErrNotFound, want: ErrDisplayUnavailable},
		{name: "removed target still marked active", paths: [][2]uint32{{0, 1}}, want: ErrDisplayUnavailable},
		{name: "inactive target", paths: [][2]uint32{{1, 0}}, want: ErrDisplayUnavailable},
		{name: "active SDR or HDR target", paths: [][2]uint32{{1, 1}}},
		{name: "another monitor remains available", paths: [][2]uint32{{0, 1}, {1, 1}}},
		{name: "console disconnected", queryErr: windows.ERROR_ACCESS_DENIED, want: ErrDisplayUnavailable},
		{name: "driver failure", queryErr: windows.ERROR_GEN_FAILURE, want: windows.ERROR_GEN_FAILURE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			paths := make([]byte, len(tc.paths)*displayConfigPathInfoSize)
			for i, values := range tc.paths {
				binary.LittleEndian.PutUint32(paths[i*displayConfigPathInfoSize+60:], values[0])
				binary.LittleEndian.PutUint32(paths[i*displayConfigPathInfoSize+68:], values[1])
			}
			err := requireActiveDisplay(func() ([]byte, uint32, error) { return paths, uint32(len(tc.paths)), tc.queryErr })
			if !errors.Is(err, tc.want) {
				t.Fatalf("error=%v, want %v", err, tc.want)
			}
			if tc.queryErr == windows.ERROR_ACCESS_DENIED && !errors.Is(err, tc.queryErr) {
				t.Fatalf("lost native error: %v", err)
			}
		})
	}
	err := requireActiveDisplay(func() ([]byte, uint32, error) { return nil, 1, nil })
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("malformed paths: %v", err)
	}
}

func TestRequireActiveDisplayNative(t *testing.T) {
	err := RequireActiveDisplay()
	if errors.Is(err, ErrDisplayUnavailable) || errors.Is(err, ErrUnsupported) {
		t.Skipf("current desktop: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Log("Windows reports an active, available display target")
}

func BenchmarkRequireActiveDisplay(b *testing.B) {
	if err := RequireActiveDisplay(); err != nil {
		b.Skip(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := RequireActiveDisplay(); err != nil {
			b.Fatal(err)
		}
	}
}
