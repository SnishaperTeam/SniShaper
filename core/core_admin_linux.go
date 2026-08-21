package core

import (
	"fmt"
	"os"
	"os/exec"
)

// isProcessElevated reports whether the current process runs as root.
// TUN and privileged system operations require root on Linux.
func isProcessElevated() bool {
	return os.Geteuid() == 0
}

// IsProcessElevated exported wrapper for main.go.
func IsProcessElevated() bool { return isProcessElevated() }

// ElevateSelf restarts the current executable through sudo. Returns nil on
// success (caller should exit; the elevated instance takes over), or an error
// if elevation was cancelled or failed.
func ElevateSelf() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	// Preserve original args (minus --elevated) and pass them to new instance
	var args []string
	for _, arg := range os.Args[1:] {
		if arg == "--elevated" {
			continue
		}
		args = append(args, arg)
	}
	args = append(args, "--elevated")

	cmd := exec.Command("sudo", append([]string{execPath}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo elevation failed: %w", err)
	}
	return nil
}