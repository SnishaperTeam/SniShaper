//go:build headless

package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"snishaper/app"
)

func opMode(args []string, out cmdOut) int {
	if len(args) == 0 {
		args = []string{"get"}
	}

	switch args[0] {
	case "get":
		a := configApp(out)
		if a == nil {
			return 1
		}
		out("当前模式: " + a.GetProxyMode())
		return 0
	case "set":
		if len(args) < 2 {
			out("用法: mode set <模式>")
			return 2
		}
		c := opRequireService(out)
		if c == nil {
			return 1
		}
		if err := c.SetProxyMode(args[1]); err != nil {
			out("切换模式失败: " + err.Error())
			return 1
		}
		out("已切换模式: " + args[1])
		return 0
	default:
		out("用法: mode get|set <模式>")
		return 2
	}
}

func opPort(args []string, out cmdOut) int {
	if len(args) == 0 {
		args = []string{"get"}
	}

	switch args[0] {
	case "get":
		a := configApp(out)
		if a == nil {
			return 1
		}
		out(fmt.Sprintf("HTTP 监听端口: %d", a.GetListenPort()))
		return 0
	case "set":
		if len(args) < 2 {
			out("用法: port set <端口>")
			return 2
		}
		port, err := strconv.Atoi(args[1])
		if err != nil || port < 1 || port > 65535 {
			out("端口无效: " + args[1])
			return 2
		}
		a := configApp(out)
		if a == nil {
			return 1
		}
		if err := a.SetListenPort(port); err != nil {
			out("设置端口失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("HTTP 监听端口已设为 %d", port))
		reloadService(out)
		return 0
	case "occupant":
		port := 0
		if len(args) >= 2 {
			value, err := strconv.Atoi(args[1])
			if err != nil {
				out("端口无效: " + args[1])
				return 2
			}
			port = value
		} else {
			a := configApp(out)
			if a == nil {
				return 1
			}
			port = a.GetListenPort()
		}
		a := configApp(out)
		if a == nil {
			return 1
		}
		occupant := a.GetPortOccupant(port)
		if occupant == nil {
			out(fmt.Sprintf("端口 %d 未被占用", port))
			return 0
		}
		out(fmt.Sprintf("端口 %d 被 PID %d 占用: %s", occupant.Port, occupant.PID, occupant.Name))
		return 0
	case "kill":
		if len(args) < 2 {
			out("用法: port kill <pid>")
			return 2
		}
		pid, err := strconv.Atoi(args[1])
		if err != nil {
			out("PID 无效: " + args[1])
			return 2
		}
		a := configApp(out)
		if a == nil {
			return 1
		}
		if err := a.KillPortOccupant(pid); err != nil {
			out("结束进程失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("已结束 PID %d", pid))
		return 0
	default:
		out("用法: port get|set <端口>|occupant [端口]|kill <pid>")
		return 2
	}
}

func opSocks5(args []string, out cmdOut) int {
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "status":
		a := configApp(out)
		if a == nil {
			return 1
		}
		state := "关"
		if a.GetSocks5Enabled() {
			state = "开"
		}
		out(fmt.Sprintf("SOCKS5: %s   监听: 127.0.0.1:%s", state, a.GetSocks5Port()))
		return 0
	case "on", "off":
		a := configApp(out)
		if a == nil {
			return 1
		}
		enabled := args[0] == "on"
		if err := a.SetSocks5Enabled(enabled); err != nil {
			out("设置 SOCKS5 失败: " + err.Error())
			return 1
		}
		out("SOCKS5 已" + map[bool]string{true: "开启", false: "关闭"}[enabled])
		return 0
	case "port":
		if len(args) < 2 {
			out("用法: socks5 port <端口>")
			return 2
		}
		a := configApp(out)
		if a == nil {
			return 1
		}
		if err := a.SetSocks5Port(args[1]); err != nil {
			out("设置 SOCKS5 端口失败: " + err.Error())
			return 1
		}
		out("SOCKS5 端口已设为 " + args[1])
		return 0
	default:
		out("用法: socks5 status|on|off|port <端口>")
		return 2
	}
}

func opMigration(args []string, out cmdOut) int {
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "status":
		a := configApp(out)
		if a == nil {
			return 1
		}
		state := "关"
		if a.GetMigrationEnabled() {
			state = "开"
		}
		server := a.GetMigrationServer()
		if strings.TrimSpace(server) == "" {
			server = "(未设置)"
		}
		out(fmt.Sprintf("迁移模式: %s   服务器: %s", state, server))
		return 0
	case "on", "off":
		a := configApp(out)
		if a == nil {
			return 1
		}
		enabled := args[0] == "on"
		if err := a.SetMigrationEnabled(enabled); err != nil {
			out("设置迁移模式失败: " + err.Error())
			return 1
		}
		out("迁移模式已" + map[bool]string{true: "开启", false: "关闭"}[enabled])
		reloadService(out)
		return 0
	case "server":
		if len(args) < 2 {
			out("用法: migration server <地址>")
			return 2
		}
		a := configApp(out)
		if a == nil {
			return 1
		}
		if err := a.SetMigrationServer(args[1]); err != nil {
			out("设置迁移服务器失败: " + err.Error())
			return 1
		}
		out("迁移服务器已设为 " + args[1])
		reloadService(out)
		return 0
	case "test":
		a := configApp(out)
		if a == nil {
			return 1
		}
		server := argAt(args, 1)
		if server == "" {
			server = a.GetMigrationServer()
		}
		if strings.TrimSpace(server) == "" {
			out("未设置迁移服务器，可用 migration test <地址> 指定")
			return 2
		}
		result, err := a.TestMigration(server)
		if err != nil {
			out("测试失败: " + err.Error())
			return 1
		}
		out("测试结果: " + result)
		return 0
	default:
		out("用法: migration status|on|off|server <地址>|test [地址]")
		return 2
	}
}

func opEvolution(args []string, out cmdOut) int {
	a := configApp(out)
	if a == nil {
		return 1
	}
	if len(args) == 0 {
		args = []string{"status"}
	}

	switch args[0] {
	case "status":
		printJSON(out, a.GetEvolutionTestStatus())
		return 0
	case "start":
		domains, enableIPv6 := parseEvolutionArgs(args[1:])
		if len(domains) == 0 {
			out("用法: evolution start <域名...> [--ipv6]")
			return 2
		}
		if c := opRequireService(out); c == nil {
			return 1
		}
		if _, err := a.StartEvolutionTest(domains, enableIPv6); err != nil {
			out("启动进化测试失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("进化测试已启动，共 %d 个域名；测试在本次进程内运行，等待结束…", len(domains)))
		return waitEvolution(a, out)
	case "stop":
		if err := a.StopEvolutionTest(); err != nil {
			out("停止测试失败: " + err.Error())
			return 1
		}
		out("已请求停止进化测试")
		return 0
	case "apply":
		if len(args) < 2 {
			out("用法: evolution apply <规则ID>")
			return 2
		}
		if err := a.ApplyEvolutionRule(args[1]); err != nil {
			out("应用规则失败: " + err.Error())
			return 1
		}
		out("已应用规则: " + args[1])
		reloadService(out)
		return 0
	default:
		out("用法: evolution status|start <域名...> [--ipv6]|stop|apply <规则ID>")
		return 2
	}
}

func parseEvolutionArgs(args []string) ([]string, bool) {
	var domains []string
	enableIPv6 := false
	for _, arg := range args {
		if arg == "--ipv6" || arg == "ipv6" {
			enableIPv6 = true
			continue
		}
		domains = append(domains, arg)
	}
	return domains, enableIPv6
}

// waitEvolution polls the tester until it finishes, because the test runs in
// this process: returning earlier would kill it together with the CLI.
func waitEvolution(a *app.App, out cmdOut) int {
	deadline := time.Now().Add(30 * time.Minute)
	lastLine := ""
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		status := a.GetEvolutionTestStatus()
		line := evolutionProgress(status)
		if line != "" && line != lastLine {
			out(line)
			lastLine = line
		}
		if evolutionFinished(status) {
			printJSON(out, status)
			return 0
		}
	}
	out("等待超时（30 分钟），测试仍在后台；可用 evolution status 查看")
	return 1
}

func evolutionProgress(status map[string]interface{}) string {
	done, _ := status["progress"].(float64)
	total, _ := status["total"].(float64)
	phase, _ := status["status"].(string)
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("进度: %.0f/%.0f  状态: %s", done, total, phase)
}

func evolutionFinished(status map[string]interface{}) bool {
	phase, _ := status["status"].(string)
	switch strings.ToLower(phase) {
	case "completed", "failed":
		return true
	}
	return false
}
