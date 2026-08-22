package app

import (
	"fmt"

	"snishaper/proxy"
)

// PortOccupant 描述占用指定端口的进程信息。
type PortOccupant struct {
	Port int    `json:"port"`
	PID  int    `json:"pid"`
	Name string `json:"name"`
}

// GetPortOccupant 返回占用指定端口的进程信息；端口未被占用时返回 nil。
// 端口落在 Windows 排除端口范围（无进程占用但无法 bind）时返回 PID 为 -1 的系统保留标记。
func (a *App) GetPortOccupant(port int) *PortOccupant {
	pid, err := proxy.FindProcessByPort(port)
	if err != nil || pid <= 0 {
		if proxy.IsPortInExcludedRange(port) {
			return &PortOccupant{Port: port, PID: -1, Name: "Windows 系统保留端口 (Excluded Port Range)"}
		}
		return nil
	}
	name, _ := proxy.GetProcessNameByPID(pid)
	return &PortOccupant{Port: port, PID: pid, Name: name}
}

// KillPortOccupant 强制结束指定 PID 的进程（含子进程树）。
func (a *App) KillPortOccupant(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	return proxy.KillProcessByPID(pid)
}
