//go:build windows

package automation

import (
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// RequireActiveDisplay verifies that Windows has an active display path whose
// target is still available. GDI monitor bounds can survive a disconnect as a
// fallback surface, so they do not establish this condition. This query does
// not depend on HDR support and does not change display or power settings.
// A connected monitor can report an active path while physically asleep; this
// checks Windows display availability, not the monitor's backlight state.
func RequireActiveDisplay() error {
	if err := procQueryDisplayConfig.Find(); err != nil {
		return ErrUnsupported
	}
	if err := procGetDisplayConfigBufferSizes.Find(); err != nil {
		return ErrUnsupported
	}
	return requireActiveDisplay(activeDisplayPaths)
}

func requireActiveDisplay(query func() ([]byte, uint32, error)) error {
	paths, count, err := query()
	if errors.Is(err, ErrNotFound) {
		return ErrDisplayUnavailable
	}
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return fmt.Errorf("%w: Windows cannot access the console display: %w", ErrDisplayUnavailable, err)
	}
	if err != nil {
		return err
	}
	if uint64(count) > uint64(len(paths)/displayConfigPathInfoSize) {
		return ErrInvalidArgument
	}
	for index := 0; index < int(count); index++ {
		path := paths[index*displayConfigPathInfoSize:]
		// DISPLAYCONFIG_PATH_INFO has a 20-byte source, a 48-byte target,
		// then flags. targetAvailable is at byte 40 within the target.
		available := binary.LittleEndian.Uint32(path[60:64]) != 0
		active := binary.LittleEndian.Uint32(path[68:72])&1 != 0
		if active && available {
			return nil
		}
	}
	return ErrDisplayUnavailable
}
