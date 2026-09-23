//go:build !linux && !headless

package app

// StartInstanceListener has no counterpart here: Windows wakes the running
// instance through its own window message path, and macOS relies on the
// runtime's single instance handling.
func (a *App) StartInstanceListener(string) {}

func (a *App) serveInstanceListener() {}

func stopInstanceListener() {}
