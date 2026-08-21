//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("cmd/emulation is Windows-only")
}
