//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func isProcessElevated() bool {
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}

// IsProcessElevated exported wrapper for main.go
func IsProcessElevated() bool { return isProcessElevated() }

// ElevateSelf restarts the current executable with administrator privileges using ShellExecute runas.
// Returns nil on success (caller should exit), or an error if elevation failed.
func ElevateSelf() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	escape := func(s string) string {
		return strings.ReplaceAll(s, "'", "''")
	}

	// Pass --elevated flag so the new instance knows it's already elevated
	psScript := fmt.Sprintf(
		"$p = Start-Process -FilePath '%s' -ArgumentList @('--elevated') -Verb RunAs -WindowStyle Hidden -PassThru; if ($null -eq $p) { exit 1 } else { exit 0 }",
		escape(execPath),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
