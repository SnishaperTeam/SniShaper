//go:build !windows

package app

func buildAutoStartCommand(execPath string, showMainWindow, autoProxy bool) string {
	return execPath
}

func setAutoStartEnabled(enabled bool, command string) error {
	return nil
}
