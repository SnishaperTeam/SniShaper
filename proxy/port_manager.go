package proxy

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// FindProcessByPort 返回占用指定端口的 PID。目前仅支持 TCP。
func FindProcessByPort(port int) (int, error) {
	if runtime.GOOS != "windows" {
		return 0, fmt.Errorf("only supported on windows")
	}

	// netstat -ano | findstr :PORT
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
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("only supported on windows")
	}

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
	if runtime.GOOS != "windows" {
		return fmt.Errorf("only supported on windows")
	}
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

// EnsurePortAvailable 检查端口占用：
// 1. 如果被 selfNames 列表中的进程占用，尝试 Kill。
// 2. 如果被其他进程占用或 Kill 失败，则寻找下一个空闲端口。
func EnsurePortAvailable(startPort int, selfNames []string) (int, error) {
	currentPort := startPort
	maxAttempts := 10 // 避免无限死循环

	for i := 0; i < maxAttempts; i++ {
		pid, err := FindProcessByPort(currentPort)
		if err != nil || pid == 0 {
			// 端口空闲，二次确认真正可用
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", currentPort))
			if err == nil {
				ln.Close()
				return currentPort, nil
			}
			// net.Listen 失败说明还是不可用，跳过找下一个
		} else {
			// 端口被占用，检查进程名
			name, _ := GetProcessNameByPID(pid)
			isSelf := false
			for _, self := range selfNames {
				if strings.EqualFold(name, self) || strings.EqualFold(name, self+".exe") {
					isSelf = true
					break
				}
			}

			if isSelf {
				// 是己方进程，尝试 Kill
				if err := KillProcessByPID(pid); err == nil {
					// 给系统一点时间回收资源
					return currentPort, nil
				}
			}
		}

		// 冲突且无法处理，尝试下一个端口
		currentPort++
	}

	return startPort, fmt.Errorf("could not find available port after %d attempts", maxAttempts)
}
