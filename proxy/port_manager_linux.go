//go:build linux

package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var ssPidPattern = regexp.MustCompile(`pid=(\d+)`)

// FindProcessByPort 返回监听指定端口的 PID。目前仅支持 TCP。
// 通过 iproute2 的 ss 查询监听 socket 的拥有进程。
func FindProcessByPort(port int) (int, error) {
	out, err := exec.Command("ss", "-ltnp", fmt.Sprintf("sport = :%d", port)).CombinedOutput()
	if err != nil {
		return 0, nil // ss 不可用或端口未被占用
	}
	for _, line := range strings.Split(string(out), "\n") {
		match := ssPidPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		pid, err := strconv.Atoi(match[1])
		if err == nil {
			return pid, nil
		}
	}
	return 0, nil
}

// GetProcessNameByPID 获取指定 PID 的进程名。
func GetProcessNameByPID(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "", fmt.Errorf("failed to read /proc/%d/comm: %w", pid, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// KillProcessByPID 强制终止指定 PID。
func KillProcessByPID(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

// IsPortInExcludedRange 在 Linux 上没有 Hyper-V/保留端口段的概念，
// 保留空实现以兼容调用方；真正可用性由 EnsurePortAvailable 的 bind 检查兜底。
func IsPortInExcludedRange(port int) bool {
	return false
}
