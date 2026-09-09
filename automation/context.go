package automation

import (
	"context"
	"errors"
)

// IsContextCancellation reports whether every non-nil leaf of an error tree
// is context.Canceled or context.DeadlineExceeded. Unlike errors.Is, it does
// not hide cleanup failures joined with cancellation. A nil error is false.
func IsContextCancellation(err error) bool {
	if err == nil {
		return false
	}
	sawCancellation := false
	var visit func(error) bool
	visit = func(current error) bool {
		if current == nil {
			return true
		}
		if multi, ok := current.(interface{ Unwrap() []error }); ok {
			children := multi.Unwrap()
			if len(children) == 0 {
				return false
			}
			for _, child := range children {
				if !visit(child) {
					return false
				}
			}
			return true
		}
		if single, ok := current.(interface{ Unwrap() error }); ok {
			return visit(single.Unwrap())
		}
		if errors.Is(current, context.Canceled) || errors.Is(current, context.DeadlineExceeded) {
			sawCancellation = true
			return true
		}
		return false
	}
	return visit(err) && sawCancellation
}
