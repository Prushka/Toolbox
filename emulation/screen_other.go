//go:build !windows

package emulation

import "context"

func CursorPosition() (int, int, error) {
	return 0, 0, ErrUnsupported
}

func (client *Client) MoveTo(context.Context, int, int) error {
	return ErrUnsupported
}
