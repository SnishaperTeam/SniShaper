package main

import (
	"context"
	"encoding/base64"
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

	"snishaper/pkg/certmanager"
	"snishaper/pkg/cfpool"
	"snishaper/proxy"
)

func NewApp() *App {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	settingsPath := resolveRuntimeFile(execDir, filepath.Join("config", "settings.json"))
	rulesPath := resolveRuntimeFile(execDir, filepath.Join("rules", "config.json"))

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
	a := &App{
		ctx:               ctx,
		cancel:            cancel,
		proxyServer:       proxy.NewProxyServer("127.0.0.1:" + port),
		ruleManager:       ruleManager,
		certPath:          filepath.Join(execDir, "cert"),
		proxyMarkerPath:   filepath.Join(execDir, "config", "system_proxy_owner.json"),
		launchedAtStartup: hasLaunchArg("--startup"),
		core:              newCoreClient(),
	}
	a.proxyServer.SetSocks5Addr("127.0.0.1:" + socks5Port)

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
	if a.core != nil {
		err := a.core.StartProxy()
		a.UpdateTrayMenu()
		a.refreshTrayMenuLater(300*time.Millisecond, time.Second)
		a.emitFrontendState()
		return err
	}
	a.proxyOpMu.Lock()
	defer a.proxyOpMu.Unlock()

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
		a.refreshTrayMenuLater(200 * time.Millisecond)
		a.appendLog("[error] StartProxy self-check failed: " + err.Error())
		return fmt.Errorf("proxy started but not listening on %s: %w", addr, err)
	}

	status := a.GetSystemProxyStatus()
	if status.Enabled && a.isManagedSystemProxy(status) {
		a.appendLog("[info] Auto-syncing system proxy configuration after port update...")
		_ = a.applySystemProxy(true, availablePort)
	}

	a.refreshTrayMenuLater(200 * time.Millisecond)
	a.appendLog("[action] StartProxy success")
	a.emitFrontendState()
	return nil
}

func (a *App) StopProxy() error {
	if a.core != nil {
		err := a.core.StopProxy()
		a.UpdateTrayMenu()
		a.refreshTrayMenuLater(300*time.Millisecond, time.Second)
		a.emitFrontendState()
		return err
	}
	a.proxyOpMu.Lock()
	defer a.proxyOpMu.Unlock()

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
	a.refreshTrayMenuLater(200 * time.Millisecond)
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

	_ = a.proxyServer.SetListenAddr(fmt.Sprintf("127.0.0.1:%d", port))

	if a.IsProxyRunning() {
		a.appendLog("[info] Port configuration updated. Restarting proxy to apply new port...")
		_ = a.StopProxy()
		time.Sleep(100 * time.Millisecond)
		if err := a.StartProxy(); err != nil {
			a.appendLog("[error] Failed to restart proxy on new port: " + err.Error())
			return err
		}
	} else {
		if a.core != nil {
			a.core.reloadIfRunning()
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
		var empty EmptyArgs
		_ = a.core.call("Core.SetSocks5Enabled", BoolReply{Value: enabled}, &empty)
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
		var empty EmptyArgs
		_ = a.core.call("Core.SetSocks5Port", StringReply{Value: port}, &empty)
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
			a.core.reloadIfRunning()
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
	captureEnabled := a.IsLogCaptureEnabled()

	err := a.core.StartTUN()
	if err == nil && captureEnabled {
		_ = a.core.StartLogCapture()
	}
	a.emitFrontendState()
	if err != nil {
		a.appendLog("[error] StartTUN failed: " + err.Error())
	}
	return err
}

func (a *App) StopTUN() error {
	if a.core != nil {
		err := a.core.StopTUN()
		a.emitFrontendState()
		if err != nil {
			a.appendLog("[error] StopTUN failed: " + err.Error())
		}
		return err
	}
	return fmt.Errorf("core client not initialized")
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
			a.core.reloadIfRunning()
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
			a.core.reloadIfRunning()
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
		a.core.reloadCertificateIfRunning()
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

func (a *App) GetStats() StatsReply {
	if a.core != nil {
		var stats StatsReply
		if err := a.core.call("Core.GetStats", EmptyArgs{}, &stats); err == nil {
			return stats
		}
	}
	down, up, etc := a.proxyServer.GetStats()
	return StatsReply{
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
	return a.ruleManager.AddSiteGroup(sg)
}

func (a *App) UpdateSiteGroup(sg proxy.SiteGroup) error {
	return a.ruleManager.UpdateSiteGroup(sg)
}

func (a *App) DeleteSiteGroup(id string) error {
	return a.ruleManager.DeleteSiteGroup(id)
}

func (a *App) GetUpstreams() []proxy.Upstream {
	return a.ruleManager.GetUpstreams()
}

func (a *App) AddUpstream(u proxy.Upstream) error {
	return a.ruleManager.AddUpstream(u)
}

func (a *App) UpdateUpstream(u proxy.Upstream) error {
	return a.ruleManager.UpdateUpstream(u)
}

func (a *App) DeleteUpstream(id string) error {
	return a.ruleManager.DeleteUpstream(id)
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
			a.core.reloadIfRunning()
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
			a.core.reloadIfRunning()
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
			a.core.reloadIfRunning()
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
	return a.ruleManager.UpsertECHProfile(p)
}

func (a *App) DeleteECHProfile(id string) error {
	return a.ruleManager.DeleteECHProfile(id)
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
	return a.ruleManager.AddDNSNode(n)
}

func (a *App) UpdateDNSNode(n proxy.DNSNode) error {
	return a.ruleManager.UpdateDNSNode(n)
}

func (a *App) DeleteDNSNode(id string) error {
	return a.ruleManager.DeleteDNSNode(id)
}

func (a *App) SetDNSNodePriority(id string, targetIndex int) error {
	return a.ruleManager.SetDNSNodePriority(id, targetIndex)
}

func (a *App) GetAutoRoutingConfig() proxy.AutoRoutingConfig {
	return a.ruleManager.GetAutoRoutingConfig()
}

func (a *App) UpdateAutoRoutingConfig(cfg proxy.AutoRoutingConfig) error {
	err := a.ruleManager.UpdateAutoRoutingConfig(cfg)
	if err == nil {
		if a.core != nil {
			a.core.reloadIfRunning()
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
			a.core.reloadIfRunning()
		}
	}
	return err
}

func (a *App) GetAppVersion() string {
	return "1.28"
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
