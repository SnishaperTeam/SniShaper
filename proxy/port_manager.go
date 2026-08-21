package proxy

import (
	"fmt"
	"net"
	"strings"
	"time"
)

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
