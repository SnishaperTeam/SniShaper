//go:build !windows && !linux && !darwin

package app

import (
	"fmt"
	"strings"
)

func buildAutoStartCommand(execPath string, showMainWindow, autoProxy bool) string {
	args := []string{execPath}
	if !showMainWindow {
		args = append(args, "--startup")
	}
	if autoProxy {
		args = append(args, "--autoproxy")
	}
	return strings.Join(args, " ")
}

// setAutoStartEnabled reports the truth on platforms without an autostart
// mechanism instead of accepting a setting that never takes effect. Disabling
// stays a no-op, because there is never an entry to remove.
func setAutoStartEnabled(enabled bool, command string) error {
	if !enabled {
		return nil
	}
	return fmt.Errorf("autostart is not supported on this platform")
}

// SetNamedAutoStartEntry is unsupported where no autostart mechanism exists.
func SetNamedAutoStartEntry(name string, enabled bool, command string) error {
	if !enabled {
		return nil
	}
	return fmt.Errorf("autostart is not supported on this platform")
}

// NamedAutoStartEntryExists is always false on these platforms.
func NamedAutoStartEntryExists(name string) bool { return false }
