//go:build !windows

package cliapp

import (
	"os/exec"
	"syscall"
)

func applyDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
