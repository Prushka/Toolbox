//go:build !windows

package hid

import "context"

// Open is intentionally unavailable outside Windows for this version.
func Open(context.Context, string, ...Option) (*Client, error) {
	return nil, ErrUnsupported
}
