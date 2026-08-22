//go:build linux || darwin

package certmanager

import "os/exec"

// Linux has no window concepts; commands run directly in the foreground.

func runHiddenCommand(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func outputHiddenCommand(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func startHiddenCommand(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

func startVisibleCommand(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

func runElevatedCommand(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}
