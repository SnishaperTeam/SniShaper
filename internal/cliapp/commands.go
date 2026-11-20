package cliapp

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"snishaper/app"
	"snishaper/common"
	"snishaper/core"
	"github.com/SnishaperTeam/snishaper-core/certmanager"
	"snishaper/pkg/sysproxy"
	corelog "github.com/SnishaperTeam/snishaper-core/log"
	"github.com/SnishaperTeam/snishaper-core/proxy"
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

func listenPort() string {
	settingsPath, _ := settingsPaths()
	rm := proxy.NewRuleManager(settingsPath, "",
		proxy.WithLogger(corelog.Std), proxy.WithRouterDebug(os.Getenv("SNISHAPER_ROUTER_DEBUG") == "1"))
	_ = rm.LoadConfig()
	port := rm.GetListenPort()
	if port == "" {
		port = "8080"
	}
	return port
}

// listenAddress 返回配置的监听地址（缺省 127.0.0.1）。
func listenAddress() string {
	settingsPath, _ := settingsPaths()
	rm := proxy.NewRuleManager(settingsPath, "",
		proxy.WithLogger(corelog.Std), proxy.WithRouterDebug(os.Getenv("SNISHAPER_ROUTER_DEBUG") == "1"))
	_ = rm.LoadConfig()
	return rm.GetListenAddress()
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
	out(fmt.Sprintf("代理已启动 (HTTP/SOCKS5 混合端口 %s)", net.JoinHostPort(listenAddress(), listenPort())))
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
	httpPort := listenPort()
	port, err := strconv.Atoi(httpPort)
	if err != nil || port < 1 {
		port = 8080
	}
	server := net.JoinHostPort(common.LocalProxyHost(listenAddress()), strconv.Itoa(port))
	if err := sysproxy.SetSystemProxy(true, server); err != nil {
		out("开启系统代理失败: " + err.Error())
		return 1
	}
	out(fmt.Sprintf("系统代理已开启 (%s)", server))
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

// statusJSON 是 `snishaper status --json` 的输出结构，也是 macOS 菜单栏
// 应用等外部工具的状态契约：服务未运行时仅 service 字段有值。
type statusJSON struct {
	Service  bool   `json:"service"`
	Proxy    *bool  `json:"proxy,omitempty"`
	Port     string `json:"port,omitempty"`
	Address  string `json:"address,omitempty"`
	Mode     string `json:"mode,omitempty"`
	TUN      *bool  `json:"tun,omitempty"`
	TUNMsg   string `json:"tun_message,omitempty"`
	SysProxy *bool  `json:"sysproxy,omitempty"`
	SysSrv   string `json:"sysproxy_server,omitempty"`
}

func opStatus(args []string, out cmdOut) int {
	c := core.NewCoreClient()
	running := c.Ping()

	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
		}
	}
	if jsonOut {
		st := statusJSON{Service: running, Port: listenPort(), Address: listenAddress()}
		if running {
			proxyOn := c.IsProxyRunning()
			st.Proxy = &proxyOn
			st.Mode = c.GetProxyMode()
			ts := c.GetTUNStatus()
			st.TUN = &ts.Running
			st.TUNMsg = ts.Message
			sp := sysproxy.GetSystemProxyStatus()
			st.SysProxy = &sp.Enabled
			st.SysSrv = sp.Server
		}
		data, err := json.Marshal(st)
		if err != nil {
			out("序列化状态失败: " + err.Error())
			return 1
		}
		out(string(data))
		return 0
	}

	out(fmt.Sprintf("服务: %s", map[bool]string{true: "运行中", false: "未运行"}[running]))
	if !running {
		return 0
	}
	out(fmt.Sprintf("代理: %s", map[bool]string{true: "开", false: "关"}[c.IsProxyRunning()]))
	out(fmt.Sprintf("混合端口 (HTTP/SOCKS5): %s", net.JoinHostPort(listenAddress(), listenPort())))
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
	case "export":
		a := configApp(out)
		if a == nil {
			return 1
		}
		content, err := a.ExportConfig()
		if err != nil {
			out("导出配置失败: " + err.Error())
			return 1
		}
		if len(args) < 2 {
			out(content)
			return 0
		}
		if err := os.WriteFile(args[1], []byte(content), 0644); err != nil {
			out("写入文件失败: " + err.Error())
			return 1
		}
		out("已导出配置: " + args[1])
		return 0
	case "import":
		if len(args) < 2 {
			out("用法: config import <path>")
			return 2
		}
		content, err := os.ReadFile(args[1])
		if err != nil {
			out("读取文件失败: " + err.Error())
			return 1
		}
		a := configApp(out)
		if a == nil {
			return 1
		}
		summary, err := a.ImportConfigWithSummary(string(content))
		if err != nil {
			out("导入配置失败: " + err.Error())
			return 1
		}
		out(fmt.Sprintf("导入完成: 共 %d，新增 %d，覆盖 %d，跳过 %d", summary.Total, summary.Added, summary.Overwritten, summary.Skipped))
		reloadService(out)
		return 0
	case "set":
		if len(args) != 3 {
			out("用法: config set <key> <value>")
			return 2
		}
		key, raw := args[1], args[2]
		val, err := parseConfigValue(key, raw, m)
		if err != nil {
			out("值类型不匹配: " + err.Error())
			return 1
		}
		if err := validateConfigValue(key, val); err != nil {
			out(err.Error())
			return 1
		}
		if !proxy.IsKnownTopLevelKey(key) {
			out("警告: 配置结构不认识键 " + key + "，仍将写入（前向兼容）")
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

// parseConfigValue 解析 `config set` 的写入值：key 已存在于配置文件时，
// 写入值强制对齐既有 JSON 类型（string→原样字符串；bool→严格 ParseBool；
// number→严格数值解析），避免把字符串键写成数字导致加载失败；新键沿用
// bool→int→float→string 的既有解析顺序。
func parseConfigValue(key, raw string, m map[string]interface{}) (interface{}, error) {
	if existing, ok := m[key]; ok {
		switch existing.(type) {
		case string:
			return raw, nil
		case bool:
			v, err := strconv.ParseBool(raw)
			if err != nil {
				return nil, fmt.Errorf("键 %s 在配置中是布尔类型，无法把 %q 解析为布尔", key, raw)
			}
			return v, nil
		case float64:
			if v, err := strconv.Atoi(raw); err == nil {
				return v, nil
			}
			if v, err := strconv.ParseFloat(raw, 64); err == nil {
				return v, nil
			}
			return nil, fmt.Errorf("键 %s 在配置中是数字类型，无法把 %q 解析为数字", key, raw)
		}
	}
	if v, err := strconv.ParseBool(raw); err == nil {
		return v, nil
	}
	if v, err := strconv.Atoi(raw); err == nil {
		return v, nil
	}
	if v, err := strconv.ParseFloat(raw, 64); err == nil {
		return v, nil
	}
	return raw, nil
}

// validateConfigValue 对已知键做值校验，非法值直接拒绝写入。
func validateConfigValue(key string, val interface{}) error {
	switch key {
	case "listen_port":
		var s string
		switch v := val.(type) {
		case string:
			s = v
		case int:
			s = strconv.Itoa(v)
		case float64:
			s = strconv.FormatFloat(v, 'f', -1, 64)
		default:
			return fmt.Errorf("listen_port 值类型无效")
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("listen_port 无效: %s（端口范围 1-65535）", s)
		}
	case "listen_address":
		s, ok := val.(string)
		if !ok {
			return fmt.Errorf("listen_address 必须是字符串")
		}
		if s != "" && net.ParseIP(s) == nil {
			return fmt.Errorf("listen_address 无效: %s（需为合法 IPv4/IPv6 地址，留空表示默认 127.0.0.1）", s)
		}
	}
	return nil
}

func opCA(args []string, out cmdOut) int {
	if len(args) == 0 {
		out("用法: ca status|install|uninstall|list|pem|export|path|regenerate")
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
	case "list":
		certs, err := cm.GetInstalledCertificates()
		if err != nil {
			out("查询已安装证书失败: " + err.Error())
			return 1
		}
		if len(certs) == 0 {
			out("系统中没有本程序安装的根证书")
			return 0
		}
		printJSON(out, certs)
		return 0
	case "pem":
		pem := cm.GetCACertPEM()
		if strings.TrimSpace(pem) == "" {
			out("读取 CA 证书失败")
			return 1
		}
		out(pem)
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

// VersionString is re-exported for the shared command layer.
func cliVersion() string {
	return app.VersionString()
}
