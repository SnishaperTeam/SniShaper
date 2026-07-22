package proxy

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// FindProcessByPort 返回占用指定端口的 PID。目前仅支持 TCP。
func FindProcessByPort(port int) (int, error) {
	if runtime.GOOS != "windows" {
		return 0, fmt.Errorf("only supported on windows")
	}

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
// 2. 如果被其他进程占用或 Kill 失败，则返回错误，不再自动跳端口。
func EnsurePortAvailable(port int, selfNames []string) (int, error) {
	pid, err := FindProcessByPort(port)
	if err == nil && pid > 0 {
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
			// 是己方进程，尝试 Kill 并等待释放
			if err := KillProcessByPID(pid); err != nil {
				return port, fmt.Errorf("port %d is occupied by self process (PID: %d) and failed to kill: %w", port, pid, err)
			}
			// 给系统短暂的时间回收套接字资源
			time.Sleep(100 * time.Millisecond)
		} else {
			return port, fmt.Errorf("port %d is occupied by process %s (PID: %d)", port, name, pid)
		}
	}

	// 二次确认套接字是否真正可用
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return port, fmt.Errorf("port %d is occupied or not available: %w", port, err)
	}
	ln.Close()

	return port, nil
}
