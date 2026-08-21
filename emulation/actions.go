package emulation

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Action is one composable keyboard/mouse operation.
type Action func(context.Context, *Client) error

// Do executes actions in order and stops at the first error.
func (client *Client) Do(ctx context.Context, actions ...Action) error {
	if ctx == nil {
		return errors.New("emulation: context is nil")
	}
	if client == nil {
		return errors.New("emulation: client is nil")
	}
	for index, action := range actions {
		if action == nil {
			err := fmt.Errorf("emulation: action %d is nil", index)
			cleanupErr := client.releaseAllBestEffort(ctx)
			return errors.Join(err, cleanupErr)
		}
		if err := action(ctx, client); err != nil {
			cleanupErr := client.releaseAllBestEffort(ctx)
			return errors.Join(fmt.Errorf("emulation: action %d: %w", index, err), cleanupErr)
		}
	}
	return nil
}

// HoldKeys creates an action that holds keys until a later ReleaseKeys action.
func HoldKeys(keys ...Key) Action {
	copied := append([]Key(nil), keys...)
	return func(ctx context.Context, client *Client) error {
		return client.KeyDown(ctx, copied...)
	}
}

// ReleaseKeys creates an action that releases keys.
func ReleaseKeys(keys ...Key) Action {
	copied := append([]Key(nil), keys...)
	return func(ctx context.Context, client *Client) error {
		return client.KeyUp(ctx, copied...)
	}
}

// TapKeys creates a key or chord action.
func TapKeys(keys ...Key) Action {
	copied := append([]Key(nil), keys...)
	return func(ctx context.Context, client *Client) error {
		return client.Press(ctx, copied...)
	}
}

// WriteText creates a US-ASCII typing action.
func WriteText(text string) Action {
	return func(ctx context.Context, client *Client) error {
		return client.Type(ctx, text)
	}
}

// MoveBy creates a relative mouse movement action.
func MoveBy(dx, dy int) Action {
	return func(ctx context.Context, client *Client) error {
		return client.Move(ctx, dx, dy)
	}
}

// JumpTo creates a Windows primary-display pixel jump action.
func JumpTo(x, y int) Action {
	return func(ctx context.Context, client *Client) error {
		return client.MoveTo(ctx, x, y)
	}
}

// MouseHold creates an action that holds mouse buttons.
func MouseHold(buttons ...Button) Action {
	copied := append([]Button(nil), buttons...)
	return func(ctx context.Context, client *Client) error {
		return client.ButtonDown(ctx, copied...)
	}
}

// MouseRelease creates an action that releases mouse buttons.
func MouseRelease(buttons ...Button) Action {
	copied := append([]Button(nil), buttons...)
	return func(ctx context.Context, client *Client) error {
		return client.ButtonUp(ctx, copied...)
	}
}

// MouseClick creates a mouse click action.
func MouseClick(buttons ...Button) Action {
	copied := append([]Button(nil), buttons...)
	return func(ctx context.Context, client *Client) error {
		return client.Click(ctx, copied...)
	}
}

// Wheel creates a two-axis scroll action.
func Wheel(vertical, horizontal int) Action {
	return func(ctx context.Context, client *Client) error {
		return client.Scroll(ctx, vertical, horizontal)
	}
}

// Pause creates a cancellable delay action.
func Pause(delay time.Duration) Action {
	return func(ctx context.Context, _ *Client) error {
		if delay < 0 {
			return errors.New("emulation: pause cannot be negative")
		}
		return waitContext(ctx, delay)
	}
}

// Repeat creates an action that repeats a group a fixed number of times.
func Repeat(count int, actions ...Action) Action {
	copied := append([]Action(nil), actions...)
	return func(ctx context.Context, client *Client) error {
		if count < 0 {
			return errors.New("emulation: repeat count cannot be negative")
		}
		for iteration := range count {
			if err := client.Do(ctx, copied...); err != nil {
				return fmt.Errorf("repeat iteration %d: %w", iteration, err)
			}
		}
		return nil
	}
}
