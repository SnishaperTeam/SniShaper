//go:build !windows

package main

func isProcessElevated() bool {
	return true
}
