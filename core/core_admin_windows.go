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

	// Preserve original args (minus --elevated) and pass them to new instance
	var args []string
	for _, arg := range os.Args[1:] {
		if arg == "--elevated" {
			continue
		}
		args = append(args, arg)
	}
	args = append(args, "--elevated")

	// Build PowerShell argument list with proper escaping
	var argParts []string
	for _, arg := range args {
		escaped := strings.ReplaceAll(arg, "'", "''")
		argParts = append(argParts, "'"+escaped+"'")
	}
	argList := strings.Join(argParts, ",")

	execPathEsc := strings.ReplaceAll(execPath, "'", "''")
	psScript := fmt.Sprintf(
		"$p = Start-Process -FilePath '%s' -ArgumentList @(%s) -Verb RunAs -WindowStyle Hidden -PassThru; if ($null -eq $p) { exit 1 } else { exit 0 }",
		execPathEsc,
		argList,
	)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
