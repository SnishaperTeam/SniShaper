//go:build windows

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func checkWintun() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve executable path: %w", err)
	}
	dllPath := filepath.Join(filepath.Dir(execPath), "wintun.dll")
	if _, err := os.Stat(dllPath); err != nil {
		return fmt.Errorf("wintun.dll check failed at %s: %w", dllPath, err)
	}
	return nil
}

func startCoreProcess(execPath string, elevated bool) error {
	if !elevated {
		cmd := exec.Command(execPath, "--core")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		return cmd.Start()
	}

	escape := func(s string) string {
		return strings.ReplaceAll(s, "'", "''")
	}

	psScript := fmt.Sprintf(
		"$p = Start-Process -FilePath '%s' -ArgumentList @('--core') -Verb RunAs -WindowStyle Hidden -PassThru; if ($null -eq $p) { exit 1 }",
		escape(execPath),
	)

	cmd := exec.Command("powershell", "-NoProfile", "-Command", psScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}
