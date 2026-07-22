//go:build !windows

package core

func isProcessElevated() bool {
	return true
}
