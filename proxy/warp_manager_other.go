//go:build !windows
package proxy

import (
	"os/exec"
)

func (m *WarpManager) setupHiddenWindow(cmd *exec.Cmd) {
	// 非 Windows 系统不需要特殊处理
}
