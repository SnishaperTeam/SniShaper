//go:build !windows

package core

import (
	"os/exec"
)

func checkWintun() error {
	return nil
}

func startCoreProcess(execPath string, _ bool) error {
	return exec.Command(execPath, "--core").Start()
}
