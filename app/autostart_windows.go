//go:build windows

package app

import (
	"fmt"
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

// SetNamedAutoStartEntry writes or removes an extra Run value under its own
// name, so the headless service can register independently of the desktop app
// entry.
func SetNamedAutoStartEntry(name string, enabled bool, command string) error {
	if name == "" {
		return fmt.Errorf("autostart entry name is empty")
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autoStartRegistryPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	if !enabled {
		if err := key.DeleteValue(name); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	if trimmed := strings.TrimSpace(command); trimmed == "" {
		return fmt.Errorf("autostart command is empty")
	}
	return key.SetStringValue(name, command)
}

// NamedAutoStartEntryExists reports whether such a Run value is registered.
func NamedAutoStartEntryExists(name string) bool {
	if name == "" {
		return false
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, autoStartRegistryPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	return err == nil && strings.TrimSpace(value) != ""
}
