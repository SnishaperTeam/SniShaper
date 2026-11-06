//go:build !windows

package app

import "os"

// NonInteractiveSession reports whether no graphical desktop is reachable:
// on Linux/macOS the GUI toolkits need DISPLAY / WAYLAND_DISPLAY. A desktop
// binary launched without either (SSH, services, CI) falls back to its
// bundled CLI mode instead of failing to create a window.
func NonInteractiveSession() bool {
	return os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == ""
}

// AttachParentConsole is meaningless without a Windows console; stdout already
// reaches the invoking terminal.
func AttachParentConsole() {}
