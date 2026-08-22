package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"snishaper/cli/app"
	"snishaper/common"
	"snishaper/core"
	"snishaper/pkg/certmanager"
	"snishaper/pkg/sysproxy"
	"snishaper/proxy"
)

type cmdOut func(string)

func opRequireService(out cmdOut) *core.CoreClient {
	c := core.NewCoreClient()
	if !c.Ping() {
		out("服务未在运行，请先启动: snishaper start")
		return nil
	}
	return c
}

func listenAddrs() (httpPort, socksPort string) {
	settingsPath, _ := settingsPaths()
	rm := proxy.NewRuleManager(settingsPath, "")
	_ = rm.LoadConfig()
	httpPort = rm.GetListenPort()
	if httpPort == "" {
		httpPort = "8080"
	}
	socksPort = rm.GetSocks5Port()
	if socksPort == "" {
		socksPort = "8081"
	}
	return httpPort, socksPort
}

func opStartProxy(out cmdOut) int {
	c := opRequireService(out)
	if c == nil {
		return 1
	}
	if err := c.StartProxy(); err != nil {
		out("启动代理失败: " + err.Error())
		return 1
	}
	httpPort, socksPort := listenAddrs()
	out(fmt.Sprintf("代理已启动 (HTTP 127.0.0.1:%s / SOCKS5 127.0.0.1:%s)", httpPort, socksPort))
	return 0
}

func opStopProxy(out cmdOut) int {
	c := opRequireService(out)
	if c == nil {
		return 1
	}
	if err := c.StopProxy(); err != nil {
		out("停止代理失败: " + err.Error())
		return 1
	}
	out("代理已停止")
	return 0
}

func opEnableSysProxy(out cmdOut) int {
	c := opRequireService(out)
	if c == nil {
		return 1
	}
	if !c.IsProxyRunning() {
		if err := c.StartProxy(); err != nil {
			out("自动启动代理失败: " + err.Error())
			return 1
		}
	}
	httpPort, _ := listenAddrs()
	port, err := strconv.Atoi(httpPort)
	if err != nil || port < 1 {
		port = 8080
	}
	if err := sysproxy.EnableSystemProxy(port); err != nil {
		out("开启系统代理失败: " + err.Error())
		return 1
	}
	out(fmt.Sprintf("系统代理已开启 (127.0.0.1:%d)", port))
	return 0
}

func opDisableSysProxy(out cmdOut) int {
	if err := sysproxy.DisableSystemProxy(); err != nil {
		out("关闭系统代理失败: " + err.Error())
		return 1
	}
	out("系统代理已关闭")
	return 0
}

func opTun(on bool, out cmdOut) int {
	c := opRequireService(out)
	if c == nil {
		return 1
	}
	var err error
	verb := "开启"
	if on {
		err = c.StartTUN()
	} else {
		verb = "关闭"
		err = c.StopTUN()
	}
	if err != nil {
		out("TUN " + verb + "失败: " + err.Error())
		return 1
	}
	out("TUN 已" + verb)
	return 0
}

func opStatus(out cmdOut) int {
	c := core.NewCoreClient()
	running := c.Ping()
	out(fmt.Sprintf("服务: %s", map[bool]string{true: "运行中", false: "未运行"}[running]))
	if !running {
		return 0
	}
	out(fmt.Sprintf("代理: %s", map[bool]string{true: "开", false: "关"}[c.IsProxyRunning()]))
	httpPort, socksPort := listenAddrs()
	out(fmt.Sprintf("HTTP: 127.0.0.1:%s    SOCKS5: 127.0.0.1:%s", httpPort, socksPort))
	out("模式: " + c.GetProxyMode())
	ts := c.GetTUNStatus()
	out(fmt.Sprintf("TUN: %s (%s)", map[bool]string{true: "开", false: "关"}[ts.Running], ts.Message))
	sp := sysproxy.GetSystemProxyStatus()
	if sp.Enabled {
		out("系统代理: 开 (" + sp.Server + ")")
	} else {
		out("系统代理: 关")
	}
	return 0
}

func opLogs(n int, out cmdOut) int {
	c := opRequireService(out)
	if c == nil {
		return 1
	}
	if n <= 0 {
		n = 100
	}
	out(c.GetRecentLogs(n))
	return 0
}

func opConfig(args []string, out cmdOut) int {
	settingsPath, _ := settingsPaths()
	if len(args) == 0 {
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			out("读取配置失败: " + err.Error())
			return 1
		}
		out(strings.TrimSpace(string(data)))
		return 0
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		out("读取配置失败: " + err.Error())
		return 1
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		out("解析配置失败: " + err.Error())
		return 1
	}
	switch args[0] {
	case "get":
		if len(args) < 2 {
			pretty, _ := json.MarshalIndent(m, "", "  ")
			out(string(pretty))
			return 0
		}
		v, ok := m[args[1]]
		if !ok {
			out("未找到配置项: " + args[1])
			return 1
		}
		raw, _ := json.Marshal(v)
		out(string(raw))
		return 0
	case "set":
		if len(args) != 3 {
			out("用法: config set <key> <value>")
			return 2
		}
		key, raw := args[1], args[2]
		var val interface{}
		if v, err := strconv.ParseBool(raw); err == nil {
			val = v
		} else if v, err := strconv.Atoi(raw); err == nil {
			val = v
		} else if v, err := strconv.ParseFloat(raw, 64); err == nil {
			val = v
		} else {
			val = raw
		}
		m[key] = val
		pretty, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			out("序列化配置失败: " + err.Error())
			return 1
		}
		if err := os.WriteFile(settingsPath, pretty, 0644); err != nil {
			out("写入配置失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("config 已设置 %s=%v", key, val))
		c := core.NewCoreClient()
		if c.Ping() {
			c.ReloadIfRunning()
			out("运行中的服务已重载配置")
		}
		return 0
	default:
		out("未知配置子命令: " + args[0])
		out("用法: config get [key] | config set <key> <value>")
		return 2
	}
}

func reloadCoreCert(out cmdOut) {
	c := core.NewCoreClient()
	if c.Ping() {
		c.ReloadCertificateIfRunning()
		out("运行中的服务已重载证书")
	}
}

func opCA(args []string, out cmdOut) int {
	if len(args) == 0 {
		out("用法: ca status|install|uninstall|export|path|regenerate")
		return 2
	}
	cm, err := certmanager.InitCertManager(common.ConfigCertDir(execDir()))
	if err != nil {
		out("初始化证书管理器失败: " + err.Error())
		return 1
	}
	switch args[0] {
	case "status":
		st := cm.GetCAInstallStatus()
		out(fmt.Sprintf("已安装: %v    平台: %s", st.Installed, st.Platform))
		out("证书路径: " + st.CertPath)
		if !st.Installed && strings.TrimSpace(st.InstallHelp) != "" {
			out("安装提示: " + st.InstallHelp)
		}
		return 0
	case "install":
		if err := cm.InstallCA(); err != nil {
			out("安装根证书失败: " + err.Error())
			return 1
		}
		out("根证书已安装到系统信任库")
		reloadCoreCert(out)
		return 0
	case "uninstall":
		certs, err := cm.GetInstalledCertificates()
		if err != nil {
			out("查询已安装证书失败: " + err.Error())
			return 1
		}
		if len(certs) == 0 {
			out("未发现本程序安装的根证书")
			return 0
		}
		for _, c := range certs {
			if err := cm.UninstallCertificate(c.Token); err != nil {
				out("卸载失败 " + c.Subject + ": " + err.Error())
			} else {
				out("已卸载: " + c.Subject)
			}
		}
		reloadCoreCert(out)
		return 0
	case "export":
		pem, err := cm.ExportCert()
		if err != nil {
			out("导出根证书失败: " + err.Error())
			return 1
		}
		dest := filepath.Join(execDir(), "ca.crt")
		if err := os.WriteFile(dest, pem, 0644); err != nil {
			out("导出失败: " + err.Error())
			return 1
		}
		out("根证书已导出: " + dest)
		return 0
	case "path":
		out(cm.GetCACertPath())
		return 0
	case "regenerate":
		if err := cm.RegenerateCA(); err != nil {
			out("重新生成根证书失败: " + err.Error())
			return 1
		}
		out("根证书已重新生成，请重新执行 ca install 安装")
		reloadCoreCert(out)
		return 0
	default:
		out("未知 CA 子命令: " + args[0])
		out("用法: ca status|install|uninstall|export|path|regenerate")
		return 2
	}
}

func printHelpText(out cmdOut) {
	out(`用法:
  snishaper                启动 TUI（日志 + 命令面板）
  snishaper start          后台启动服务
  snishaper stop           停止正在运行的服务
  snishaper status         查看服务/代理/系统代理/TUN 状态
  snishaper logs [N]       打印最近 N 行日志（默认 100）
  snishaper proxy on|off   启动/停止代理
  snishaper sysproxy on|off
                           开启/关闭系统代理
  snishaper tun on|off     切换 TUN 模式（需要管理员/root）
  snishaper config get [key]
  snishaper config set <key> <value>
                           查看/修改 settings.json
  snishaper ca status      查看根证书安装状态
  snishaper ca install     安装根证书到系统信任库（需要管理员）
  snishaper ca uninstall   卸载已安装的根证书
  snishaper ca export      导出 CA 证书到 ca.crt
  snishaper ca path        显示 CA 证书文件路径
  snishaper ca regenerate  重新生成根证书（之后需重新安装）
  snishaper version        打印版本号
  snishaper help           显示本帮助`)
}

// VersionString is re-exported for the shared command layer.
func cliVersion() string {
	return app.VersionString()
}
