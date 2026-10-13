//go:build !windows

package singtun

// cleanupStaleAdapters 在非 Windows 平台为 no-op，返回 0 以对齐签名。
func cleanupStaleAdapters(logf func(string)) int {
	_ = logf
	return 0
}
