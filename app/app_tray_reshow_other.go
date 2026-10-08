//go:build !windows || headless

package app

// requestTrayReshow is a no-op on any build that does not use the Windows GUI
// tray implementation: non-Windows platforms, and every headless build. The
// whole reshow mechanism exists only to work around Wails' Windows tray
// lifecycle (see app_tray_reshow_windows.go).
func requestTrayReshow() {}