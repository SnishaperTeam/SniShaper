//go:build windows

package proxy

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// FindProcessByPort 返回占用指定端口的 PID。目前仅支持 TCP。
func FindProcessByPort(port int) (int, error) {
	// netstat -ano | findstr :PORT
	// SAFE: port is int, so fmt.Sprintf with %d cannot be injected
	cmd := exec.Command("cmd", "/c", fmt.Sprintf("netstat -ano | findstr :%d", port))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, nil // 没找到通常意味着端口未被占用
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// TCP    0.0.0.0:8080           0.0.0.0:0              LISTENING       pid
		fields := strings.Fields(line)
		if len(fields) >= 5 && strings.Contains(fields[1], fmt.Sprintf(":%d", port)) {
			pid, err := strconv.Atoi(fields[len(fields)-1])
			if err == nil {
				return pid, nil
			}
		}
	}
	return 0, nil
}

// GetProcessNameByPID 获取指定 PID 的进程名。
func GetProcessNameByPID(pid int) (string, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Image Name                     PID Session Name        Session#    Mem Usage
	// ========================= ======== ================ =========== ============
	// snishaper.exe                13012 Console                    1     12,345 K
	line := strings.TrimSpace(string(out))
	if strings.Contains(line, "No tasks are running") {
		return "", fmt.Errorf("process not found")
	}
	fields := strings.Fields(line)
	if len(fields) > 0 {
		return fields[0], nil
	}
	return "", fmt.Errorf("failed to parse tasklist output")
}

// KillProcessByPID 强制终止指定 PID 及其子进程。
func KillProcessByPID(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

// IsPortInExcludedRange 检查端口是否落在 Windows 系统排除端口范围
// （Hyper-V/WSL/Docker/WinNAT 等保留段）。落在其中的端口 bind 会报
// WSAEACCES（forbidden by its access permissions），且 netstat 查不到占用进程。
func IsPortInExcludedRange(port int) bool {
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "excludedportrange", "protocol=tcp")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		start, err1 := strconv.Atoi(fields[0])
		end, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		if port >= start && port <= end {
			return true
		}
	}
	return false
}
