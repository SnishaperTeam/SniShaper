//go:build !windows && !linux

package proxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// FindProcessByPort returns the PID listening on the given TCP port using
// lsof, which is available by default on macOS and most BSD systems.
func FindProcessByPort(port int) (int, error) {
	out, err := exec.Command("lsof", "-nP", "-iTCP", fmt.Sprintf(":%d", port), "-sTCP:LISTEN").CombinedOutput()
	if err != nil {
		return 0, nil
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if pid, err := strconv.Atoi(fields[1]); err == nil {
				return pid, nil
			}
		}
	}
	return 0, nil
}

// GetProcessNameByPID returns the process name for the given PID using ps.
func GetProcessNameByPID(pid int) (string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to read process name for pid %d: %w", pid, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// KillProcessByPID forcefully terminates the given PID.
func KillProcessByPID(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

// IsPortInExcludedRange is a no-op on non-Windows platforms; actual
// availability is verified by the bind check in EnsurePortAvailable.
func IsPortInExcludedRange(port int) bool {
	return false
}
