//go:build linux

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildAutoStartCommand builds the Exec line for the XDG autostart entry.
func buildAutoStartCommand(execPath string, showMainWindow, autoProxy bool) string {
	args := []string{execPath, "--startup"}
	if autoProxy {
		args = append(args, "--autoproxy")
	}
	return strings.Join(args, " ")
}

// autostartDir resolves the invoking user's XDG autostart directory. When the
// app runs elevated (root via sudo), write to $SUDO_USER's autostart so the
// entry appears in the user's desktop session.
func autostartDir() string {
	user := os.Getenv("SUDO_USER")
	home := ""
	if user != "" && user != "root" {
		home = "/home/" + user
	} else {
		home, _ = os.UserHomeDir()
	}
	if os.Getenv("XDG_CONFIG_HOME") != "" {
		return filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "autostart")
	}
	return filepath.Join(home, ".config", "autostart")
}

// setAutoStartEnabled writes or removes the XDG autostart .desktop entry.
func setAutoStartEnabled(enabled bool, command string) error {
	dir := autostartDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}
	entry := filepath.Join(dir, "snishaper.desktop")
	if !enabled {
		return os.Remove(entry)
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=SniShaper\n" +
		"Comment=SniShaper system proxy\n" +
		"Exec=" + command + "\n" +
		"Terminal=false\n" +
		"NoDisplay=true\n" +
		"X-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(entry, []byte(content), 0644)
}
