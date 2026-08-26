package automation

import (
	"errors"
	"sync"
	"testing"
)

func TestBeginPowerRequestValidation(t *testing.T) {
	for _, options := range []PowerRequestOptions{
		{},
		{SystemRequired: true},
		{Reason: "missing requirement"},
	} {
		if _, err := BeginPowerRequest(options); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("BeginPowerRequest(%+v) error = %v, want ErrInvalidArgument", options, err)
		}
	}
}

func TestPowerRequestCloseIsConcurrentAndIdempotent(t *testing.T) {
	calls := 0
	request := &PowerRequest{closeFunc: func() error {
		calls++
		return nil
	}}
	var wait sync.WaitGroup
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := request.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	wait.Wait()
	if calls != 1 {
		t.Fatalf("close calls = %d, want 1", calls)
	}
	if err := (*PowerRequest)(nil).Close(); err != nil {
		t.Fatalf("nil Close error = %v", err)
	}
}
