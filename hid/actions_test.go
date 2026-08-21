package hid

import (
	"context"
	"errors"
	"testing"
)

func TestActionsAndRepeat(t *testing.T) {
	harness := newClientHarness(t, nil, WithTapDelay(0))
	actions := []Action{
		HoldKeys(Ctrl),
		TapKeys(MustKey('a')),
		ReleaseKeys(Ctrl),
		Repeat(2, MouseClick(ButtonLeft)),
	}
	if err := harness.client.Do(context.Background(), actions...); err != nil {
		t.Fatal(err)
	}
	assertCommand(t, harness.next(t), opKeyDown, []byte{byte(Ctrl)})
	assertCommand(t, harness.next(t), opKeyDown, []byte{'a'})
	assertCommand(t, harness.next(t), opKeyUp, []byte{'a'})
	assertCommand(t, harness.next(t), opKeyUp, []byte{byte(Ctrl)})
	for range 2 {
		assertCommand(t, harness.next(t), opMouseDown, []byte{byte(ButtonLeft)})
		assertCommand(t, harness.next(t), opMouseUp, []byte{byte(ButtonLeft)})
	}
}

func TestDoStopsAtError(t *testing.T) {
	harness := newClientHarness(t, nil)
	sentinel := errors.New("stop")
	err := harness.client.Do(context.Background(),
		func(context.Context, *Client) error { return sentinel },
		func(context.Context, *Client) error { t.Fatal("second action ran"); return nil },
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Do error = %v, want sentinel", err)
	}
	assertCommand(t, harness.next(t), opReleaseAll, nil)
}
