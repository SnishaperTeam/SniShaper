//go:build !windows

package core

import "fmt"

func isProcessElevated() bool {
	return true
}

func IsProcessElevated() bool { return isProcessElevated() }

func ElevateSelf() error {
	return fmt.Errorf("elevation not supported on this platform")
}
