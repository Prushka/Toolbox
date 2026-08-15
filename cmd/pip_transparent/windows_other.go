//go:build !windows

package main

import "errors"

func togglePiPWindows() ([]toggleResult, error) {
	return nil, errors.New("this program only supports Windows")
}
