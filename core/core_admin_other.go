//go:build !windows && !linux

package core

import (
	"fmt"
	"os"
)

// isProcessElevated reports whether the process runs as root. The remaining
// desktop platforms expose no elevation query, so the effective user id is the
// only honest signal.
func isProcessElevated() bool {
	return os.Geteuid() == 0
}

func IsProcessElevated() bool { return isProcessElevated() }

// ElevateSelf has no implementation outside Windows: the app has to be started
// with the privileges it needs (sudo or an administrator shell).
func ElevateSelf() error {
	return fmt.Errorf("automatic elevation is not supported on this platform; run SniShaper with sudo")
}
