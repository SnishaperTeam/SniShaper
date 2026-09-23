package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"snishaper/common"
	"snishaper/core"
)

// DiagnosticCheck is one item of a self check.
type DiagnosticCheck struct {
	Name       string `json:"name"`
	OK         bool   `json:"ok"`
	Skipped    bool   `json:"skipped,omitempty"`
	Detail     string `json:"detail,omitempty"`
	DurationMS int64  `json:"duration_ms,omitempty"`
}

// ProxyDiagnostics is a read only snapshot of the runtime state, meant for bug
// reports and the diagnostics panel.
type ProxyDiagnostics struct {
	Version        string                `json:"version"`
	Platform       string                `json:"platform"`
	Elevated       bool                  `json:"elevated"`
	ServiceRunning bool                  `json:"service_running"`
	CorePID        int                   `json:"core_pid,omitempty"`
	CoreExecutable string                `json:"core_executable,omitempty"`
	ProxyRunning   bool                  `json:"proxy_running"`
	ListenPort     int                   `json:"listen_port"`
	ProxyMode      string                `json:"proxy_mode"`
	Socks5Enabled  bool                  `json:"socks5_enabled"`
	Socks5Port     string                `json:"socks5_port"`
	SystemProxy    systemProxyDiagnostic `json:"system_proxy"`
	TUN            proxyTUNDiagnostic    `json:"tun"`
	IPv6Available  bool                  `json:"ipv6_available"`
	SettingsPath   string                `json:"settings_path,omitempty"`
	RulesPath      string                `json:"rules_path,omitempty"`
	CertPath       string                `json:"cert_path,omitempty"`
	LogDir         string                `json:"log_dir,omitempty"`
	LogFiles       int                   `json:"log_files"`
}

type systemProxyDiagnostic struct {
	Enabled  bool   `json:"enabled"`
	Server   string `json:"server,omitempty"`
	Override string `json:"override,omitempty"`
	Managed  bool   `json:"managed"`
}

type proxyTUNDiagnostic struct {
	Supported bool   `json:"supported"`
	Running   bool   `json:"running"`
	Enabled   bool   `json:"enabled"`
	Driver    string `json:"driver,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ProxySelfCheckResult carries the outcome of ProxySelfCheck.
type ProxySelfCheckResult struct {
	OK     bool              `json:"ok"`
	Checks []DiagnosticCheck `json:"checks"`
}

// GetProxyDiagnostics collects the state a bug report needs. Every field is
// best effort: values that need a running core fall back to their zero value.
func (a *App) GetProxyDiagnostics() ProxyDiagnostics {
	diag := ProxyDiagnostics{
		Version:       VersionString(),
		Platform:      runtime.GOOS + "/" + runtime.GOARCH,
		Elevated:      core.IsProcessElevated(),
		ProxyRunning:  a.IsProxyRunning(),
		ListenPort:    a.GetListenPort(),
		ProxyMode:     a.GetProxyMode(),
		Socks5Enabled: a.GetSocks5Enabled(),
		Socks5Port:    a.GetSocks5Port(),
		CertPath:      a.certPath,
		LogDir:        a.logDir,
	}

	c := core.NewCoreClient()
	diag.ServiceRunning = c.Ping()
	if diag.ServiceRunning {
		var info core.CoreInfoReply
		if err := c.Call("Core.GetInfo", core.EmptyArgs{}, &info); err == nil {
			diag.CorePID = info.PID
			diag.CoreExecutable = info.Executable
		}
	}

	status := a.GetSystemProxyStatus()
	diag.SystemProxy = systemProxyDiagnostic{
		Enabled:  status.Enabled,
		Server:   status.Server,
		Override: status.Override,
		Managed:  status.Enabled && a.isManagedSystemProxy(status),
	}

	tun := a.GetTUNStatus()
	diag.TUN = proxyTUNDiagnostic{
		Supported: tun.Supported,
		Running:   tun.Running,
		Enabled:   tun.Enabled,
		Driver:    tun.Driver,
		Message:   tun.Message,
	}

	diag.IPv6Available = a.GetIPv6Available()
	diag.LogFiles = len(a.GetLogFiles())

	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		diag.SettingsPath = common.ConfigSettingsPath(execDir)
		diag.RulesPath = common.ConfigRulesPath(execDir)
	}

	return diag
}

// ProxySelfCheck runs a short end to end check of the local proxy: the service
// answers, the listeners are open, a request through the proxy succeeds, and the
// system proxy and TUN states match the configuration.
func (a *App) ProxySelfCheck() ProxySelfCheckResult {
	result := ProxySelfCheckResult{}

	c := core.NewCoreClient()
	serviceUp := c.Ping()
	result.Checks = append(result.Checks, DiagnosticCheck{
		Name:   "service",
		OK:     serviceUp,
		Detail: map[bool]string{true: "核心服务可达", false: "服务未运行，请先执行 snishaper start"}[serviceUp],
	})

	httpPort := a.GetListenPort()
	httpAddr := fmt.Sprintf("127.0.0.1:%d", httpPort)
	listening := false
	if ok, detail, elapsed := dialCheck(httpAddr, 2*time.Second); ok {
		listening = true
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "http_listen", OK: true, Detail: "HTTP 端口 " + httpAddr + " 可连接", DurationMS: elapsed})
	} else {
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "http_listen", OK: false, Detail: detail, DurationMS: elapsed})
	}

	if a.GetSocks5Enabled() {
		socksAddr := "127.0.0.1:" + a.GetSocks5Port()
		ok, detail, elapsed := dialCheck(socksAddr, 2*time.Second)
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "socks5_listen", OK: ok, Detail: detailOr(detail, "SOCKS5 端口 "+socksAddr+" 可连接"), DurationMS: elapsed})
	} else {
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "socks5_listen", OK: true, Skipped: true, Detail: "SOCKS5 未启用"})
	}

	if listening {
		ok, detail, elapsed := httpThroughProxy(httpAddr)
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "http_via_proxy", OK: ok, Detail: detail, DurationMS: elapsed})
	} else {
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "http_via_proxy", OK: false, Skipped: true, Detail: "代理未在监听，跳过"})
	}

	status := a.GetSystemProxyStatus()
	switch {
	case !status.Enabled:
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "system_proxy", OK: true, Skipped: true, Detail: "系统代理未开启"})
	case a.isManagedSystemProxy(status):
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "system_proxy", OK: true, Detail: "系统代理指向本程序: " + status.Server})
	default:
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "system_proxy", OK: false, Detail: "系统代理已开启但不是本程序设置的: " + status.Server})
	}

	cfg := a.GetTUNConfig()
	tun := a.GetTUNStatus()
	switch {
	case tun.Running:
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "tun", OK: true, Detail: "TUN 运行中（驱动 " + tun.Driver + "）"})
	case cfg.Enabled:
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "tun", OK: false, Detail: "配置已启用 TUN 但未运行: " + tun.Message})
	default:
		result.Checks = append(result.Checks, DiagnosticCheck{Name: "tun", OK: true, Skipped: true, Detail: "TUN 未启用"})
	}

	result.OK = true
	for _, check := range result.Checks {
		if !check.OK {
			result.OK = false
			break
		}
	}
	return result
}

func dialCheck(addr string, timeout time.Duration) (bool, string, int64) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return false, fmt.Sprintf("无法连接 %s: %v", addr, err), elapsed
	}
	_ = conn.Close()
	return true, "", elapsed
}

func detailOr(detail, fallback string) string {
	if strings.TrimSpace(detail) == "" {
		return fallback
	}
	return detail
}

// httpThroughProxy fetches a small endpoint through the local proxy, which is
// the only way to prove that requests actually leave the machine.
func httpThroughProxy(proxyAddr string) (bool, string, int64) {
	target := "https://www.gstatic.com/generate_204"
	proxyURL, err := url.Parse("http://" + proxyAddr)
	if err != nil {
		return false, "解析代理地址失败: " + err.Error(), 0
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false, "构造请求失败: " + err.Error(), 0
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return false, fmt.Sprintf("经代理访问 %s 失败: %v", target, err), elapsed
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, fmt.Sprintf("经代理访问 %s 成功（HTTP %d）", target, resp.StatusCode), elapsed
	}
	return false, fmt.Sprintf("经代理访问 %s 返回 HTTP %d", target, resp.StatusCode), elapsed
}
