//go:build windows

package app

import (
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	autoStartRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	autoStartValueName    = "SniShaper"
)

// buildAutoStartCommand builds the registry Run command.
// --startup is added only when the main window should stay hidden on auto-start;
// --autoproxy is added when the proxy should auto-enable on auto-start.
// Manual launches (no args) always show the main window.
func buildAutoStartCommand(execPath string, showMainWindow, autoProxy bool) string {
	trimmed := strings.TrimSpace(execPath)
	if trimmed == "" {
		return ""
	}
	cmd := syscall.EscapeArg(trimmed)
	if !showMainWindow {
		cmd += " --startup"
	}
	if autoProxy {
		cmd += " --autoproxy"
	}
	return cmd
}

func setAutoStartEnabled(enabled bool, command string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autoStartRegistryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enabled {
		err = key.DeleteValue(autoStartValueName)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}

	return key.SetStringValue(autoStartValueName, command)
}
