//go:build !windows || headless

package app

// startSessionWatcher 在非 Windows 平台是 no-op。
func (a *App) startSessionWatcher() {}

// isSessionLocked 在非 Windows 平台始终返回 false。
func isSessionLocked() bool { return false }