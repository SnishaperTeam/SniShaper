//go:build !windows

package app

func buildAutoStartCommand(execPath string) string {
	return execPath
}

func setAutoStartEnabled(enabled bool, command string) error {
	return nil
}
