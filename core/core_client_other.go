//go:build !windows

package core

import (
	"os/exec"
)

func startCoreProcess(execPath string, _ bool) error {
	return exec.Command(execPath, "--core").Start()
}
