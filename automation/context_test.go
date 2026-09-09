package automation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestContextCancellationRetainsOtherFailures(t *testing.T) {
	cleanup := errors.New("release failed")
	for _, test := range []struct {
		err  error
		want bool
	}{
		{nil, false}, {context.Canceled, true}, {context.DeadlineExceeded, true},
		{fmt.Errorf("worker: %w", context.Canceled), true},
		{errors.Join(context.Canceled, context.DeadlineExceeded), true},
		{errors.Join(context.Canceled, cleanup), false},
		{fmt.Errorf("worker: %w", errors.Join(context.Canceled, cleanup)), false},
		{cleanup, false},
	} {
		if got := IsContextCancellation(test.err); got != test.want {
			t.Errorf("IsContextCancellation(%v) = %v, want %v", test.err, got, test.want)
		}
	}
}
