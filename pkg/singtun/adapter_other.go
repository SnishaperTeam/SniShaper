//go:build !windows

package singtun

func cleanupStaleAdapters(logf func(string)) {
	_ = logf
}
