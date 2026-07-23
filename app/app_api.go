package app

import (
	"context"

	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"snishaper/common"
	"snishaper/core"
	"snishaper/pkg/certmanager"
	"snishaper/pkg/cfpool"
	"snishaper/pkg/dohresolver"
	"snishaper/proxy"
)

func NewApp() *App {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	settingsPath := common.ResolveRuntimeFile(execDir, filepath.Join("config", "settings.json"))
	rulesPath := common.ResolveRuntimeFile(execDir, filepath.Join("rules", "config.json"))

	ruleManager := proxy.NewRuleManager(settingsPath, rulesPath)
	if err := ruleManager.LoadConfig(); err != nil {
		log.Printf("[warn] Failed to load config at init: %v", err)
	}

	port := ruleManager.GetListenPort()
	if port == "" {
		port = "8080"
	}

	socks5Port := ruleManager.GetSocks5Port()
	if socks5Port == "" {
		socks5Port = "8081"
	}

	ctx, cancel := context.WithCancel(context.Background())
	proxyServer := proxy.NewProxyServer("127.0.0.1:" + port)
	a := &App{
		ctx:               ctx,
		cancel:            cancel,
		proxyServer:       proxyServer,
		ruleManager:       ruleManager,
		certPath:          filepath.Join(execDir, "cert"),
		proxyMarkerPath:   filepath.Join(execDir, "config", "system_proxy_owner.json"),
		launchedAtStartup: HasLaunchArg("--startup"),
		core:              core.NewCoreClient(),
	}
	a.proxyServer.SetSocks5Addr("127.0.0.1:" + socks5Port)

	// Auto-restart when proxy server stops unexpectedly
	a.proxyServer.OnStop = func(err error) {
		a.appendLog("[error] Proxy server stopped unexpectedly: " + err.Error())
		a.UpdateTrayMenu()
		a.RunSafeAsync("proxy auto-restart", func() {
			time.Sleep(2 * time.Second)
			a.proxyOpMu.Lock()
			defer a.proxyOpMu.Unlock()
			if !a.proxyServer.IsRunning() {
				if err2 := a.proxyServer.Start(); err2 != nil {
					a.appendLog("[error] Proxy auto-restart failed: " + err2.Error())
				} else {
					a.appendLog("[action] Proxy auto-restarted successfully")
				}
				a.UpdateTrayMenu()
				a.emitFrontendState()
			}
		})
	}

	// Set CF pool refresh callback — called async when pool is stale (>1 day)
	a.proxyServer.SetCFRefreshCallback(func() {
		a.RefreshCloudflareIPPool()
	})

	// Initialize Cloudflare IP pool and trigger background health check on startup
	cf := ruleManager.GetCloudflareConfig()
	if len(cf.PreferredIPs) > 0 {
		a.proxyServer.UpdateCloudflareIPPool(cf.PreferredIPs)
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			select {
			case <-time.After(1 * time.Second): // Wait for app to stabilize
				a.proxyServer.TriggerCFHealthCheck()
			case <-a.ctx.Done():
				return
			}
		}()
	}

	// Initialize auto router (needed for GFW list refresh even without core)
	ruleManager.InitAutoRouter(a.proxyServer.GetDoHResolver())

	return a
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's Wails v3!", name)
}

func (a *App) StartProxy() error {
	a.proxyOpMu.Lock()
	defer a.proxyOpMu.Unlock()

	if a.core != nil {
		err := a.core.StartProxy()
		a.UpdateTrayMenu()
		a.emitFrontendState()
		return err
	}

	a.appendLog("[action] StartProxy called")

	originalPort := a.GetListenPort()
	if originalPort == 0 {
		originalPort = 8080
	}

	availablePort, err := proxy.EnsurePortAvailable(originalPort, []string{"snishaper", "usque"})
	if err != nil {
		a.appendLog(fmt.Sprintf("[warn] Port probe failed: %v, attempting with original port", err))
		availablePort = originalPort
	}

	if availablePort != originalPort {
		a.appendLog(fmt.Sprintf("[info] Port %d was occupied. Switched to %d.", originalPort, availablePort))
		if err := a.SetListenPort(availablePort); err != nil {
			a.appendLog("[warn] Failed to update config with new port: " + err.Error())
		}
	}

	a.proxyServer.SetSocks5Enabled(true)
	a.ruleManager.SetSocks5Enabled(true)
	_ = a.ruleManager.SaveConfig()

	socks5OriginalPort := a.ruleManager.GetSocks5Port()
	if socks5OriginalPort == "" {
		socks5OriginalPort = "8081"
	}
	socks5PortNum, err := strconv.Atoi(socks5OriginalPort)
	if err == nil {
		socks5Available, err := proxy.EnsurePortAvailable(socks5PortNum, []string{"snishaper", "usque"})
		if err != nil {
			a.appendLog(fmt.Sprintf("[warn] SOCKS5 port probe failed: %v, using original port", err))
			socks5Available = socks5PortNum
		}
		if socks5Available != socks5PortNum {
			a.appendLog(fmt.Sprintf("[info] SOCKS5 port %d was occupied. Switched to %d.", socks5PortNum, socks5Available))
			a.ruleManager.SetSocks5Port(strconv.Itoa(socks5Available))
		}
		a.proxyServer.SetSocks5Addr(fmt.Sprintf("127.0.0.1:%d", socks5Available))
	}

	a.syncCFPoolNAT64Prefix()
	err = a.proxyServer.Start()
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "Only one usage of each socket address") || strings.Contains(msg, "bind: address already in use") {
			msg += " (核心启动失败：端口仍被占用，请检查权限或手动杀进程)"
		}
		a.appendLog("[error] StartProxy failed: " + msg)
		return fmt.Errorf("%s", msg)
	}

	a.UpdateTrayMenu()
	addr := a.proxyServer.GetListenAddr()
	if err := a.waitForProxyListen(addr, 2*time.Second); err != nil {
		_ = a.proxyServer.Stop()
		a.appendLog("[error] StartProxy self-check failed: " + err.Error())
		return fmt.Errorf("proxy started but not listening on %s: %w", addr, err)
	}

	status := a.GetSystemProxyStatus()
	if status.Enabled && a.isManagedSystemProxy(status) {
		a.appendLog("[info] Auto-syncing system proxy configuration after port update...")
		_ = a.applySystemProxy(true, availablePort)
	}

	a.appendLog("[action] StartProxy success")
	a.emitFrontendState()
	return nil
}

func (a *App) StopProxy() error {
	a.proxyOpMu.Lock()
	defer a.proxyOpMu.Unlock()

	if a.core != nil {
		err := a.core.StopProxy()
		a.UpdateTrayMenu()
		a.emitFrontendState()
		return err
	}

	a.appendLog("[action] StopProxy called")

	var errs []error
	status := a.GetSystemProxyStatus()
	if status.Enabled && a.isManagedSystemProxy(status) {
		a.appendLog("[info] Disabling system proxy as proxy core is stopping...")
		if err := a.applySystemProxy(false, 0); err != nil {
			errs = append(errs, err)
		}
	}

	if err := a.proxyServer.Stop(); err != nil {
		errs = append(errs, err)
	}

	a.UpdateTrayMenu()
	a.emitFrontendState()

	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		joinedErr := strings.Join(msgs, "; ")
		a.appendLog("[error] StopProxy failed: " + joinedErr)
		return fmt.Errorf("%s", joinedErr)
	}

	a.appendLog("[action] StopProxy success")
	return nil
}

func (a *App) IsProxyRunning() bool {
	if a.core != nil {
		return a.core.IsProxyRunning()
	}
	return a.proxyServer.IsRunning()
}

func (a *App) GetListenPort() int {
	portStr := a.ruleManager.GetListenPort()
	if portStr == "" {
		return 8080
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 8080
	}
	return port
}

func (a *App) restartProxy() error {
	a.proxyOpMu.Lock()
	defer a.proxyOpMu.Unlock()

	a.appendLog("[action] restartProxy called")

	if a.core != nil {
		if err := a.core.StopProxy(); err != nil {
			return err
		}
		if err := a.core.StartProxy(); err != nil {
			return err
		}
		a.UpdateTrayMenu()
		a.emitFrontendState()
		return nil
	}

	if err := a.proxyServer.Stop(); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if err := a.proxyServer.Start(); err != nil {
		return err
	}
	a.UpdateTrayMenu()
	a.emitFrontendState()
	return nil
}

func (a *App) SetListenPort(port int) error {
	a.appendLog(fmt.Sprintf("[action] SetListenPort called: %d", port))
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d", port)
	}

	oldPort := a.GetListenPort()
	if oldPort == port {
		return nil
	}

	a.ruleManager.SetListenPort(strconv.Itoa(port))
	_ = a.ruleManager.SaveConfig()

	_ = a.proxyServer.SetListenAddr(fmt.Sprintf("127.0.0.1:%d", port))

	if a.IsProxyRunning() {
		a.appendLog("[info] Port configuration updated. Restarting proxy to apply new port...")
		if err := a.restartProxy(); err != nil {
			a.appendLog("[error] Failed to restart proxy on new port: " + err.Error())
			return err
		}
	} else {
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}

	return nil
}

func (a *App) GetSocks5Enabled() bool {
	return a.ruleManager.GetSocks5Enabled()
}

func (a *App) SetSocks5Enabled(enabled bool) error {
	a.appendLog(fmt.Sprintf("[action] SetSocks5Enabled: %v", enabled))
	a.proxyServer.SetSocks5Enabled(enabled)
	a.ruleManager.SetSocks5Enabled(enabled)
	_ = a.ruleManager.SaveConfig()
	if a.core != nil {
		var empty core.EmptyArgs
		_ = a.core.Call("Core.SetSocks5Enabled", core.BoolReply{Value: enabled}, &empty)
	}
	return nil
}

func (a *App) GetSocks5Port() string {
	return a.ruleManager.GetSocks5Port()
}

func (a *App) SetSocks5Port(port string) error {
	a.appendLog(fmt.Sprintf("[action] SetSocks5Port: %s", port))
	a.ruleManager.SetSocks5Port(port)
	_ = a.ruleManager.SaveConfig()
	a.proxyServer.SetSocks5Addr("127.0.0.1:" + port)
	if a.core != nil {
		var empty core.EmptyArgs
		_ = a.core.Call("Core.SetSocks5Port", core.StringReply{Value: port}, &empty)
	}
	return nil
}

func (a *App) GetProxyMode() string {
	if a.core != nil {
		return a.core.GetProxyMode()
	}
	return a.proxyServer.GetMode()
}

func (a *App) SetProxyMode(mode string) error {
	a.appendLog(fmt.Sprintf("[action] SetProxyMode: %s", mode))
	if a.core != nil {
		err := a.core.SetProxyMode(mode)
		a.emitFrontendState()
		return err
	}
	err := a.proxyServer.SetMode(mode)
	if err == nil {
		a.emitFrontendState()
	}
	return err
}

func (a *App) GetLanguage() string {
	return a.ruleManager.GetLanguage()
}

func (a *App) SetLanguage(lang string) error {
	a.appendLog(fmt.Sprintf("[action] SetLanguage called: %s", lang))
	return a.ruleManager.SetLanguage(lang)
}

func (a *App) GetTheme() string {
	return a.ruleManager.GetTheme()
}

func (a *App) SetTheme(theme string) error {
	a.appendLog(fmt.Sprintf("[action] SetTheme called: %s", theme))
	return a.ruleManager.SetTheme(theme)
}

func (a *App) GetTUNConfig() proxy.TUNConfig {
	return a.ruleManager.GetTUNConfig()
}

func (a *App) UpdateTUNConfig(cfg proxy.TUNConfig) error {
	a.appendLog("[action] UpdateTUNConfig called")
	err := a.ruleManager.UpdateTUNConfig(cfg)
	if err == nil {
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}
	return err
}

func (a *App) GetTUNStatus() proxy.TUNStatus {
	if a.core != nil {
		return a.core.GetTUNStatus()
	}
	return proxy.TUNStatus{
		Running: false,
		Message:   "core_service_not_running",
	}
}

func (a *App) StartTUN() error {
	if a.core == nil {
		return fmt.Errorf("core client not initialized")
	}

	a.appendLog("[action] StartTUN called")

	// Disable managed system proxy before TUN to avoid port/resource conflicts
	status := a.GetSystemProxyStatus()
	if status.Enabled && a.isManagedSystemProxy(status) {
		a.appendLog("[action] StartTUN: disabling managed system proxy before TUN")
		if err := a.applySystemProxy(false, 0); err != nil {
			a.appendLog("[warn] StartTUN: failed to disable system proxy: " + err.Error())
		}
		a.tunRestoreSysProxy = true
	}

	captureEnabled := a.IsLogCaptureEnabled()

	a.runSafeAsync("start TUN", func() {
		err := a.core.StartTUN()
		if err == nil && captureEnabled {
			_ = a.core.StartLogCapture()
		}
		if err != nil {
			a.appendLog("[error] StartTUN failed: " + err.Error())
		}
		a.emitFrontendState()
	})
	return nil
}

func (a *App) StopTUN() error {
	if a.core == nil {
		return fmt.Errorf("core client not initialized")
	}

	err := a.core.StopTUN()
	if err != nil {
		a.appendLog("[error] StopTUN failed: " + err.Error())
	}

	// Restore system proxy if it was disabled for TUN
	if a.tunRestoreSysProxy {
		a.tunRestoreSysProxy = false
		port := a.GetListenPort()
		a.appendLog(fmt.Sprintf("[action] StopTUN: restoring managed system proxy on :%d", port))
		if err2 := a.applySystemProxy(true, port); err2 != nil {
			a.appendLog("[error] StopTUN: failed to restore system proxy: " + err2.Error())
		}
	}

	a.emitFrontendState()
	return err
}

func (a *App) ExportCert() string {
	if a.certManager == nil {
		return ""
	}
	data, err := a.certManager.ExportCert()
	if err != nil {
		a.appendLog("Export cert error: " + err.Error())
		return ""
	}
	return string(data)
}

func (a *App) ExportConfig() (string, error) {
	return a.ruleManager.ExportConfig()
}

func (a *App) ImportConfig(content string) error {
	a.appendLog("[action] ImportConfig called")
	err := a.ruleManager.ImportConfig(content)
	if err == nil {
		a.proxyServer.UpdateCloudflareConfig(a.ruleManager.GetCloudflareConfig())
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
		a.UpdateTrayMenu()
	}
	return err
}

func (a *App) ImportConfigWithSummary(content string) (proxy.ImportSummary, error) {
	a.appendLog("[action] ImportConfigWithSummary called")
	summary, err := a.ruleManager.ImportConfigWithSummary(content)
	if err == nil {
		a.proxyServer.UpdateCloudflareConfig(a.ruleManager.GetCloudflareConfig())
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
		a.UpdateTrayMenu()
	}
	return summary, err
}

func (a *App) InstallCA() error {
	if a.certManager == nil {
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] InstallCA called")
	if err := a.certManager.InstallCA(); err != nil {
		a.appendLog("[cert] InstallCA failed: " + err.Error())
		return err
	}
	a.appendLog("[cert] InstallCA succeeded")
	return nil
}

func (a *App) OpenCAFile() error {
	if a.certManager == nil {
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] OpenCAFile called")
	return a.certManager.OpenCAFile()
}

func (a *App) OpenCertDir() error {
	if a.certManager == nil {
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] OpenCertDir called")
	return a.certManager.OpenCertDir()
}

func (a *App) RegenerateCert() error {
	if a.certManager == nil {
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] RegenerateCert called")
	if err := a.certManager.RegenerateCA(); err != nil {
		a.appendLog("[cert] RegenerateCert failed: " + err.Error())
		return err
	}
	if a.core != nil {
		a.core.ReloadCertificateIfRunning()
	}
	a.appendLog("[cert] RegenerateCert succeeded")
	return nil
}

func (a *App) UninstallCert(thumbprint string) error {
	if a.certManager == nil {
		a.appendLog("[cert] UninstallCert failed: cert manager not initialized")
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] UninstallCert called: " + thumbprint)
	if err := a.certManager.UninstallCertificate(thumbprint); err != nil {
		a.appendLog("[cert] UninstallCert failed: " + err.Error())
		return err
	}
	a.appendLog("[cert] UninstallCert succeeded: " + thumbprint)
	return nil
}

func (a *App) GetRecentLogs(limit int) string {
	if a.core != nil {
		if logs := a.core.GetRecentLogs(limit); strings.TrimSpace(logs) != "" {
			return logs
		}
	}
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}

	if a.logBuffer != nil {
		lines := a.logBuffer.Snapshot(limit)
		if len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}

	return ""
}

func (a *App) ClearLogs() error {
	if a.core != nil {
		_ = a.core.ClearLogs()
	}
	if a.logBuffer != nil {
		a.logBuffer.Clear()
	}
	a.appendLog("[action] Logs cleared")
	return nil
}

func (a *App) GetStats() core.StatsReply {
	if a.core != nil {
		var stats core.StatsReply
		if err := a.core.Call("Core.GetStats", core.EmptyArgs{}, &stats); err == nil {
			return stats
		}
	}
	down, up, etc := a.proxyServer.GetStats()
	return core.StatsReply{
		Down: down,
		Up:   up,
		Etc:  etc,
	}
}

func (a *App) GetCACertPath() string {
	if a.certManager == nil && a.certPath != "" {
		if cm, err := certmanager.InitCertManager(a.certPath); err == nil {
			a.certManager = cm
		}
	}
	if a.certManager != nil {
		return a.certManager.GetCACertPath()
	}
	return ""
}

func (a *App) GetCACertPEM() string {
	if a.certManager != nil {
		return a.certManager.GetCACertPEM()
	}
	return ""
}

func (a *App) GetCAInstallStatus() CAInstallStatus {
	if a.certManager == nil {
		if a.certPath != "" {
			if cm, err := certmanager.InitCertManager(a.certPath); err == nil {
				a.certManager = cm
			}
		}
	}
	if a.certManager == nil {
		return CAInstallStatus{
			CertPath:    a.certPath,
			Platform:    "windows",
			InstallHelp: "证书状态初始化中",
		}
	}
	status := a.certManager.GetCAInstallStatus()
	return CAInstallStatus{
		Installed:   status.Installed,
		Platform:    status.Platform,
		CertPath:    status.CertPath,
		InstallHelp: status.InstallHelp,
	}
}

func (a *App) GetInstalledCerts() []certmanager.InstalledCert {
	if a.certManager == nil {
		a.appendLog("[cert] GetInstalledCerts failed: cert manager not initialized")
		return []certmanager.InstalledCert{}
	}
	a.appendLog("[cert] GetInstalledCerts called")
	certs, err := a.certManager.GetInstalledCertificates()
	if err != nil {
		a.appendLog("GetInstalledCertificates error: " + err.Error())
		return []certmanager.InstalledCert{}
	}
	a.appendLog(fmt.Sprintf("[cert] GetInstalledCerts succeeded: %d certs", len(certs)))
	return certs
}

func (a *App) GetSiteGroups() []proxy.SiteGroup {
	return a.ruleManager.GetSiteGroups()
}

func (a *App) AddSiteGroup(sg proxy.SiteGroup) error {
	err := a.ruleManager.AddSiteGroup(sg)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) UpdateSiteGroup(sg proxy.SiteGroup) error {
	err := a.ruleManager.UpdateSiteGroup(sg)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) DeleteSiteGroup(id string) error {
	err := a.ruleManager.DeleteSiteGroup(id)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) GetUpstreams() []proxy.Upstream {
	return a.ruleManager.GetUpstreams()
}

func (a *App) AddUpstream(u proxy.Upstream) error {
	err := a.ruleManager.AddUpstream(u)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) UpdateUpstream(u proxy.Upstream) error {
	err := a.ruleManager.UpdateUpstream(u)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) DeleteUpstream(id string) error {
	err := a.ruleManager.DeleteUpstream(id)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) GetCloudflareConfig() proxy.CloudflareConfig {
	return a.ruleManager.GetCloudflareConfig()
}

func (a *App) UpdateCloudflareConfig(cfg proxy.CloudflareConfig) error {
	oldCfg := a.ruleManager.GetCloudflareConfig()

	err := a.ruleManager.UpdateCloudflareConfig(cfg)
	if err == nil {
		a.proxyServer.UpdateCloudflareConfig(cfg)
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
		if cfg.AutoUpdate && !oldCfg.AutoUpdate {
			a.appendLog("[Cloudflare] Auto update enabled, triggering fetch...")
			go a.RefreshCloudflareIPPool()
		}
		a.UpdateTrayMenu()
	}
	return err
}

func (a *App) GetCloudflareIPStats() []*cfpool.IPStats {
	return a.proxyServer.GetAllCFIPsWithStats()
}

func (a *App) RefreshCloudflareIPPool() {
	cfg := a.ruleManager.GetCloudflareConfig()
	ips, err := cfpool.FetchCloudflareIPs(cfg.APIKey)
	if err != nil {
		log.Printf("[Cloudflare] Failed to fetch preferred IPs: %v", err)
		a.appendLog("[error] Cloudflare 优选 IP 获取失败: " + err.Error())
		return
	}

	if len(ips) > 0 {
		log.Printf("[Cloudflare] Successfully fetched %d preferred IPs", len(ips))
		a.appendLog(fmt.Sprintf("[success] 成功获取 %d 个 Cloudflare 优选 IP", len(ips)))

		a.proxyServer.UpdateCloudflareIPPool(ips)
		a.proxyServer.SetCFPoolFetchTime(time.Now())
		cfg.PreferredIPs = ips
		_ = a.ruleManager.UpdateCloudflareConfig(cfg)
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}
}

func (a *App) ForceFetchCloudflareIPs() error {
	cfg := a.ruleManager.GetCloudflareConfig()
	ips, err := cfpool.FetchCloudflareIPs(cfg.APIKey)
	if err != nil {
		log.Printf("[Cloudflare] Failed to fetch preferred IPs: %v", err)
		a.appendLog("[error] 手动获取失败: " + err.Error())
		return err
	}

	if len(ips) > 0 {
		log.Printf("[Cloudflare] Successfully fetched %d preferred IPs", len(ips))
		a.appendLog(fmt.Sprintf("[success] 成功获取 %d 个 Cloudflare 优选 IP", len(ips)))
		a.proxyServer.UpdateCloudflareIPPool(ips)
		a.proxyServer.SetCFPoolFetchTime(time.Now())
		cfg.PreferredIPs = ips
		_ = a.ruleManager.UpdateCloudflareConfig(cfg)
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
		a.proxyServer.TriggerCFHealthCheck()
	}
	return nil
}

func (a *App) RemoveInvalidCFIPs() error {
	a.appendLog("[Cloudflare] Removing invalid/slow IPs...")
	a.proxyServer.RemoveInvalidCFIPs()
	return nil
}

func (a *App) TriggerCFHealthCheck() error {
	a.appendLog("[Cloudflare] Manual health check triggered...")
	a.proxyServer.TriggerCFHealthCheck()
	return nil
}

func (a *App) GetECHProfiles() []proxy.ECHProfile {
	return a.ruleManager.GetECHProfiles()
}

func (a *App) UpsertECHProfile(p proxy.ECHProfile) error {
	err := a.ruleManager.UpsertECHProfile(p)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) DeleteECHProfile(id string) error {
	err := a.ruleManager.DeleteECHProfile(id)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) FetchECHConfig(domain string, dohURL string) (string, error) {
	a.appendLog(fmt.Sprintf("[DoH] Fetching ECH for %s via %s", domain, dohURL))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	config, err := a.proxyServer.FetchECH(ctx, domain, dohURL)
	if err != nil {
		a.appendLog(fmt.Sprintf("[error] ECH fetch failed: %v", err))
		return "", err
	}

	if len(config) == 0 {
		return "", fmt.Errorf("no ECH config found")
	}

	encoded := base64.StdEncoding.EncodeToString(config)
	a.appendLog(fmt.Sprintf("[success] ECH fetch ok (%d bytes)", len(config)))
	return encoded, nil
}

func (a *App) GetDNSNodes() []proxy.DNSNode {
	return a.ruleManager.GetDNSNodes()
}

func (a *App) AddDNSNode(n proxy.DNSNode) error {
	err := a.ruleManager.AddDNSNode(n)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) UpdateDNSNode(n proxy.DNSNode) error {
	err := a.ruleManager.UpdateDNSNode(n)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) DeleteDNSNode(id string) error {
	err := a.ruleManager.DeleteDNSNode(id)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

func (a *App) SetDNSNodePriority(id string, targetIndex int) error {
	err := a.ruleManager.SetDNSNodePriority(id, targetIndex)
	if err == nil && a.core != nil {
		a.core.ReloadIfRunning()
	}
	return err
}

type DNSNodeTestResult struct {
	Success bool   `json:"success"`
	Latency int64  `json:"latency"`
	Error   string `json:"error,omitempty"`
	IPs     []string `json:"ips,omitempty"`
}

func (a *App) TestDNSNode(id string) (DNSNodeTestResult, error) {
	nodes := a.ruleManager.GetDNSNodes()
	var node *proxy.DNSNode
	for i := range nodes {
		if nodes[i].ID == id {
			node = &nodes[i]
			break
		}
	}
	if node == nil {
		return DNSNodeTestResult{Success: false, Error: "node not found"}, nil
	}

	resolver := a.proxyServer.GetDoHResolver()
	if resolver == nil {
		return DNSNodeTestResult{Success: false, Error: "DNS resolver not initialized"}, nil
	}

	// Convert proxy.DNSNode to dohresolver.DNSNode
	dohNode := dohresolver.DNSNode{
		Name:          node.Name,
		URL:           node.URL,
		SNI:           node.SNI,
		IPs:           node.IPs,
		ECHEnabled:    node.ECHEnabled,
		ECHProfileID:  node.ECHProfileID,
		ECHAutoUpdate: node.ECHAutoUpdate,
		QUIC:          node.QUIC,
		Enabled:       node.Enabled,
		CertVerify: dohresolver.CertVerifyConfig{
			Mode:                  node.CertVerify.Mode,
			Names:                 node.CertVerify.Names,
			Suffixes:              node.CertVerify.Suffixes,
			SPKISHA256:            node.CertVerify.SPKISHA256,
			AllowUnknownAuthority: node.CertVerify.AllowUnknownAuthority,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	start := time.Now()
	ips, err := resolver.TestNode(ctx, dohNode)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return DNSNodeTestResult{Success: false, Latency: latency, Error: err.Error()}, nil
	}

	return DNSNodeTestResult{Success: true, Latency: latency, IPs: ips}, nil
}

func (a *App) GetAutoRoutingConfig() proxy.AutoRoutingConfig {
	return a.ruleManager.GetAutoRoutingConfig()
}

func (a *App) UpdateAutoRoutingConfig(cfg proxy.AutoRoutingConfig) error {
	err := a.ruleManager.UpdateAutoRoutingConfig(cfg)
	if err == nil {
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}
	return err
}

func (a *App) GetAutoRoutingStatus() proxy.GFWListStatus {
	return a.ruleManager.GetAutoRoutingStatus()
}

func (a *App) RefreshGFWList() error {
	a.appendLog("[action] RefreshGFWList called")
	_, err := a.ruleManager.RefreshGFWList()
	if err != nil {
		a.appendLog("[error] RefreshGFWList failed: " + err.Error())
	} else {
		a.appendLog("[action] RefreshGFWList success")
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}
	return err
}

func (a *App) GetAppVersion() string {
	return "1.29"
}

func (a *App) CheckUpdate() CheckUpdateResult {
	updateURL := "https://gh.llkk.cc/https://raw.githubusercontent.com/dongzheyu/SniShaperWeb/master/update.txt"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(updateURL)
	if err != nil {
		errorMsg := err.Error()
		a.appendLog("[update] Failed to check update: " + errorMsg)

		errorDetail := "check_failed"
		if strings.Contains(errorMsg, "timeout") || strings.Contains(errorMsg, "deadline exceeded") {
			errorDetail = "network_timeout"
		} else if strings.Contains(errorMsg, "connection refused") {
			errorDetail = "connection_refused"
		} else if strings.Contains(errorMsg, "no such host") || strings.Contains(errorMsg, "DNS") {
			errorDetail = "dns_error"
		} else if strings.Contains(errorMsg, "proxy") {
			errorDetail = "proxy_error"
		}

		return CheckUpdateResult{
			HasUpdate:   false,
			Message:     "check_failed",
			ErrorDetail: errorDetail,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		a.appendLog("[update] Failed to read update info: " + err.Error())
		return CheckUpdateResult{
			HasUpdate: false,
			Message:   "check_failed",
		}
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 {
		a.appendLog("[update] Invalid update info format")
		return CheckUpdateResult{
			HasUpdate: false,
			Message:   "check_failed",
		}
	}

	latestVersion := strings.TrimSpace(lines[0])
	downloadURL := strings.TrimSpace(lines[1])
	currentVersion := a.GetAppVersion()

	a.appendLog(fmt.Sprintf("[update] Current: %s, Latest: %s", currentVersion, latestVersion))

	comparison := compareVersions(currentVersion, latestVersion)

	switch comparison {
	case -1:
		return CheckUpdateResult{
			HasUpdate:     true,
			LatestVersion: latestVersion,
			DownloadURL:   downloadURL,
			Message:       "update_available",
		}
	case 0:
		return CheckUpdateResult{
			HasUpdate:     false,
			LatestVersion: latestVersion,
			DownloadURL:   downloadURL,
			Message:       "up_to_date",
		}
	default:
		return CheckUpdateResult{
			HasUpdate:     false,
			LatestVersion: latestVersion,
			DownloadURL:   downloadURL,
			Message:       "dev_version",
		}
	}
}

func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int

		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &num1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &num2)
		}

		if num1 < num2 {
			return -1
		} else if num1 > num2 {
			return 1
		}
	}

	return 0
}

func (a *App) OpenURL(rawURL string) {
	if rawURL == "" {
		return
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		a.appendLog(fmt.Sprintf("[update] Invalid or unsupported URL: %s", rawURL))
		return
	}
	cmd := exec.Command("cmd", "/c", "start", parsed.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		a.appendLog(fmt.Sprintf("[update] Failed to open URL: %v", err))
	}
}

func (a *App) QuitApp() {
	a.shouldQuit = true
	a.shutdown()
	if a.mainWindow != nil {
		a.mainWindow.Close()
	}
	if a.wailsApp != nil {
		a.wailsApp.Quit()
	}
}

func (a *App) waitForProxyListen(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond) // Faster dial
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond) // Faster retry
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return lastErr
}

func (a *App) syncCFPoolNAT64Prefix() {
	if a.proxyServer != nil && a.proxyServer.GetCFPool() != nil {
		profiles := a.ruleManager.GetNAT64Profiles()
		var defaultPrefix string
		for _, p := range profiles {
			if strings.TrimSpace(p.Prefix) != "" {
				defaultPrefix = p.Prefix
				break
			}
		}
		a.proxyServer.GetCFPool().SetNAT64Prefix(defaultPrefix)
	}
}

func (a *App) GetNAT64Profiles() []proxy.NAT64Profile {
	return a.ruleManager.GetNAT64Profiles()
}

func (a *App) AddNAT64Profile(p proxy.NAT64Profile) error {
	err := a.ruleManager.AddNAT64Profile(p)
	if err == nil {
		a.syncCFPoolNAT64Prefix()
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}
	return err
}

func (a *App) UpdateNAT64Profile(p proxy.NAT64Profile) error {
	err := a.ruleManager.UpdateNAT64Profile(p)
	if err == nil {
		a.syncCFPoolNAT64Prefix()
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}
	return err
}

func (a *App) DeleteNAT64Profile(id string) error {
	err := a.ruleManager.DeleteNAT64Profile(id)
	if err == nil {
		a.syncCFPoolNAT64Prefix()
		if a.core != nil {
			a.core.ReloadIfRunning()
		}
	}
	return err
}

func (a *App) TestNAT64Profile(prefix string) (int64, error) {
	ips, err := net.LookupIP("www.cloudflare.com")
	if err != nil || len(ips) == 0 {
		return 0, fmt.Errorf("DNS lookup failed: %w", err)
	}

	var mappedIP string
	for _, ip := range ips {
		if ip.To4() != nil {
			mapped, ok := mapNAT64AddrForTest(ip.String(), prefix)
			if ok {
				mappedIP = mapped
				break
			}
		}
	}

	if mappedIP == "" {
		return 0, fmt.Errorf("no IPv4 address available for mapping")
	}

	target := net.JoinHostPort(mappedIP, "443")
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	start := time.Now()
	conn, err := dialer.Dial("tcp", target)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	return time.Since(start).Milliseconds(), nil
}

func mapNAT64AddrForTest(ipStr string, prefix string) (string, bool) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ipStr, true
	}
	parsedIP := net.ParseIP(ipStr)
	if parsedIP == nil {
		return ipStr, true
	}
	ipv4 := parsedIP.To4()
	if ipv4 == nil {
		return ipStr, false
	}

	var prefixIP net.IP
	if strings.Contains(prefix, "/") {
		_, ipnet, err := net.ParseCIDR(prefix)
		if err == nil && ipnet != nil {
			prefixIP = ipnet.IP
		}
	} else {
		prefixIP = net.ParseIP(prefix)
	}

	if prefixIP == nil || len(prefixIP) != 16 {
		return ipStr, true
	}
	mappedIP := make(net.IP, 16)
	copy(mappedIP, prefixIP[:12])
	copy(mappedIP[12:], ipv4)
	return mappedIP.String(), true
}

func (a *App) GetMigrationEnabled() bool {
	return a.ruleManager.GetMigrationEnabled()
}

func (a *App) SetMigrationEnabled(enabled bool) error {
	a.appendLog(fmt.Sprintf("[action] SetMigrationEnabled: %v", enabled))
	return a.ruleManager.SetMigrationEnabled(enabled)
}

func (a *App) GetMigrationServer() string {
	return a.ruleManager.GetMigrationServer()
}

func (a *App) SetMigrationServer(server string) error {
	a.appendLog(fmt.Sprintf("[action] SetMigrationServer: %s", server))
	return a.ruleManager.SetMigrationServer(server)
}

func (a *App) TestMigration(server string) (string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", fmt.Errorf("migration server URL is empty")
	}
	a.appendLog(fmt.Sprintf("[action] TestMigration: %s", server))

	// Test by connecting to cloudflare.com:443
	testTarget := "cloudflare.com:443"
	apiURL := fmt.Sprintf("%s?target=%s&ip=1.1.1.1", server, testTarget)
	a.appendLog(fmt.Sprintf("[Migration] GET %s", apiURL))

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response to check if we got a valid ticket
	var result struct {
		Ticket      string `json:"ticket"`
		CipherSuite uint16 `json:"cipher_suite"`
		Version     uint16 `json:"version"`
		TargetIP    string `json:"target_ip"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("invalid response: %v", err)
	}

	if result.Ticket == "" {
		return "", fmt.Errorf("no session ticket in response")
	}

	msg := fmt.Sprintf("Session ticket acquired! IP: %s, Cipher: 0x%04X, Version: 0x%04X",
		result.TargetIP, result.CipherSuite, result.Version)
	a.appendLog(fmt.Sprintf("[Migration] Test success: %s", msg))
	return msg, nil
}
