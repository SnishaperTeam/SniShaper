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
		if err := os.Remove(entry); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
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

// SetNamedAutoStartEntry writes or removes an extra autostart entry under its
// own file name, so the headless service can register independently of the
// desktop app entry.
func SetNamedAutoStartEntry(name string, enabled bool, command string) error {
	if name == "" {
		return fmt.Errorf("autostart entry name is empty")
	}
	dir := autostartDir()
	if !enabled {
		if err := os.Remove(filepath.Join(dir, name+".desktop")); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create autostart dir: %w", err)
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=" + name + "\n" +
		"Comment=SniShaper command line service\n" +
		"Exec=" + command + "\n" +
		"Terminal=false\n" +
		"NoDisplay=true\n" +
		"X-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(filepath.Join(dir, name+".desktop"), []byte(content), 0644)
}

// NamedAutoStartEntryExists reports whether such an entry is registered.
func NamedAutoStartEntryExists(name string) bool {
	if name == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(autostartDir(), name+".desktop"))
	return err == nil
}
