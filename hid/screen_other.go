//go:build !windows

package hid

import "context"

func CursorPosition() (int, int, error) {
	return 0, 0, ErrUnsupported
}

func (client *Client) MoveTo(context.Context, int, int) error {
	return ErrUnsupported
}

func (client *Client) MoveToRelative(context.Context, int, int) error {
	return ErrUnsupported
}

func (client *Client) ClickAt(context.Context, int, int, ...Button) error {
	return ErrUnsupported
}

func (client *Client) MoveToAbsoluteScreen(context.Context, int, int) error {
	return ErrUnsupported
}

func (client *Client) MoveToWindow(context.Context, int, int, int, int) error {
	return ErrUnsupported
}

func (client *Client) ClickAtWindow(context.Context, int, int, int, int, ...Button) error {
	return ErrUnsupported
}

func (client *Client) ClickPreparedWindow(context.Context, int, int, int, int, ...Button) error {
	return ErrUnsupported
}
