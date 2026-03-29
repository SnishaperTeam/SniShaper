package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"snishaper/cert"
	"snishaper/proxy"
	"snishaper/sysproxy"
)

type App struct {
	wailsApp    *application.App
	mainWindow  *application.WebviewWindow
	proxyServer *proxy.ProxyServer
	certManager *cert.CertManager
	ruleManager *proxy.RuleManager
	warpMgr     *proxy.WarpManager
	certPath    string
	logPath     string
	logFile     *os.File
	logBuffer   *ringLogWriter
	shouldQuit  bool
	systemTray  *application.SystemTray
	trayMenuV3  *application.Menu
	proxyItemV3 *application.MenuItem
	warpItemV3  *application.MenuItem
	systemProxyItemV3 *application.MenuItem
	proxyOpMu   sync.Mutex
	warpOpMu    sync.Mutex
}

type ringLogWriter struct {
	mu      sync.Mutex
	lines   []string
	pending string
	max     int
}

func newRingLogWriter(max int) *ringLogWriter {
	if max <= 0 {
		max = 1000
	}
	return &ringLogWriter{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (w *ringLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	text := w.pending + strings.ReplaceAll(string(p), "\r\n", "\n")
	parts := strings.Split(text, "\n")
	if len(parts) == 0 {
		return len(p), nil
	}
	w.pending = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		if line == "" {
			continue
		}
		w.lines = append(w.lines, line)
		if len(w.lines) > w.max {
			w.lines = w.lines[len(w.lines)-w.max:]
		}
	}
	return len(p), nil
}

func (w *ringLogWriter) Snapshot(limit int) []string {
	if limit <= 0 {
		limit = 200
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	total := len(w.lines)
	if total == 0 {
		return []string{}
	}
	if limit > total {
		limit = total
	}
	start := total - limit
	out := make([]string, limit)
	copy(out, w.lines[start:])
	return out
}

func (w *ringLogWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = w.lines[:0]
	w.pending = ""
}

func (w *ringLogWriter) AppendLine(line string) {
	if line == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = append(w.lines, line)
	if len(w.lines) > w.max {
		w.lines = w.lines[len(w.lines)-w.max:]
	}
}

func NewApp() *App {
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	configPath := resolveRuntimeFile(execDir, filepath.Join("rules", "config.json"))

	ruleManager := proxy.NewRuleManager(configPath)
	if err := ruleManager.LoadConfig(); err != nil {
		log.Printf("[warn] Failed to load config at init: %v", err)
	}

	port := ruleManager.GetListenPort()
	if port == "" {
		port = "8080"
	}

	return &App{
		proxyServer: proxy.NewProxyServer("127.0.0.1:" + port),
		ruleManager: ruleManager,
		warpMgr:     proxy.NewWarpManager(execDir),
		certPath:    filepath.Join(execDir, "cert"),
		logPath:     filepath.Join(execDir, "snishaper.log"),
	}
}

func resolveRuntimeFile(execDir, relativePath string) string {
	return filepath.Join(execDir, relativePath)
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.startupV3()
	return nil
}

func (a *App) ServiceShutdown() error {
	a.shutdown()
	return nil
}

func (a *App) startupV3() {
	a.setupFileLogger()
	log.Printf("[startup] SniShaper startup hook entered")
	a.appendLog("[startup] in-memory log channel ready")

	var err error
	a.certManager, err = cert.InitCertManager(a.certPath)
	if err != nil {
		a.appendLog("[startup] Failed to init cert manager: " + err.Error())
	} else {
		a.appendLog("[startup] Cert manager initialized: " + a.certPath)
	}

	if err := a.ruleManager.LoadConfig(); err != nil {
		a.appendLog("[startup] Failed to load config: " + err.Error())
	}

	a.proxyServer.SetRuleManager(a.ruleManager)
	a.proxyServer.UpdateCloudflareConfig(a.ruleManager.GetCloudflareConfig())
	a.proxyServer.SetCertGenerator(a.certManager)
	a.proxyServer.SetWarpManager(a.warpMgr)
	a.proxyServer.SetLogCallback(a.appendLog)
	a.warpMgr.SetLogCallback(a.appendLog)

	if err := sysproxy.SaveOriginalProxySettings(); err != nil {
		a.appendLog("[startup] Failed to save original proxy settings: " + err.Error())
	}

	a.appendLog("[startup] SniShaper started successfully")

	// Auto-start proxy for easier diagnostics and better UX
	go func() {
		a.UpdateTrayMenu()
		time.Sleep(500 * time.Millisecond)
		_ = a.StartProxy()

		// If auto update is enabled, fetch IPs immediately
		cfg := a.ruleManager.GetCloudflareConfig()
		if cfg.AutoUpdate {
			a.appendLog("[Cloudflare] Auto update is enabled, fetching initial IPs...")
			go a.RefreshCloudflareIPPool()
		}

		if cfg.WarpEnabled {
			a.appendLog("[Warp] Auto start is enabled, starting Warp...")
			_ = a.warpMgr.Start()
		}
		a.emitFrontendState()
	}()

	if a.mainWindow != nil {
		a.mainWindow.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
			if !a.shouldQuit && a.GetCloseToTray() {
				event.Cancel()
				a.mainWindow.Hide()
			}
		})
	}
}

func (a *App) beforeClose() bool {
	if !a.shouldQuit && a.GetCloseToTray() {
		a.mainWindow.Hide()
		return true // Cancel the close event
	}
	return false // Allow the close event
}

// SetTrayMenu is no longer needed in v3 as it's setup in main.go

func (a *App) QuitApp() {
	a.shouldQuit = true
	a.wailsApp.Quit()
}

func (a *App) RevealMainWindow() {
	if a.mainWindow == nil {
		return
	}
	a.mainWindow.Restore()
	a.mainWindow.Show()
	a.mainWindow.Focus()
}

func (a *App) HandleWindowClose() {
	if a.GetCloseToTray() && !a.shouldQuit && a.mainWindow != nil {
		a.mainWindow.Hide()
		return
	}
	a.QuitApp()
}

func (a *App) GetCloseToTray() bool {
	if a.ruleManager == nil {
		return true
	}
	return a.ruleManager.GetCloseToTray()
}

func (a *App) SetCloseToTray(enabled bool) error {
	if a.ruleManager == nil {
		return fmt.Errorf("RuleManager not initialized")
	}
	return a.ruleManager.SetCloseToTray(enabled)
}

func (a *App) UpdateTrayMenu() {
	proxyRunning := a.proxyServer != nil && a.proxyServer.IsRunning()
	warpRunning := a.warpMgr != nil && a.warpMgr.GetStatus().Running
	systemProxyEnabled := a.GetSystemProxyStatus().Enabled

	application.InvokeAsync(func() {
		if a.proxyItemV3 != nil {
			a.proxyItemV3.SetChecked(proxyRunning)
		}
		if a.warpItemV3 != nil {
			a.warpItemV3.SetChecked(warpRunning)
		}
		if a.systemProxyItemV3 != nil {
			if systemProxyEnabled {
				a.systemProxyItemV3.SetLabel("系统代理: 开")
			} else {
				a.systemProxyItemV3.SetLabel("系统代理: 关")
			}
		}
	})
}

func (a *App) refreshTrayMenuLater(delays ...time.Duration) {
	for _, delay := range delays {
		go func(delay time.Duration) {
			if delay > 0 {
				time.Sleep(delay)
			}
			a.UpdateTrayMenu()
		}(delay)
	}
}

func (a *App) runSafeAsync(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.appendLog(fmt.Sprintf("[panic] %s: %v\n%s", name, r, string(debug.Stack())))
			}
		}()
		fn()
	}()
}

func (a *App) emitFrontendState() {
	if a.mainWindow == nil {
		return
	}

	state := map[string]any{
		"proxyRunning":       a.proxyServer != nil && a.proxyServer.IsRunning(),
		"systemProxyEnabled": a.GetSystemProxyStatus().Enabled,
		"warpRunning":        a.warpMgr != nil && a.warpMgr.GetStatus().Running,
	}

	application.InvokeAsync(func() {
		a.mainWindow.EmitEvent("app:state", state)
	})
}

func (a *App) shutdown() {
	a.appendLog("[shutdown] SniShaper shutting down...")

	if a.proxyServer.IsRunning() {
		a.appendLog("[shutdown] Stopping proxy server...")
		if err := a.proxyServer.Stop(); err != nil {
			a.appendLog("[shutdown] Failed to stop proxy: " + err.Error())
		}
	}

	if a.warpMgr != nil {
		a.appendLog("[shutdown] Stopping Warp manager...")
		_ = a.warpMgr.Stop()
	}

	a.appendLog("[shutdown] Restoring original system proxy settings...")
	if err := sysproxy.RestoreOriginalProxySettings(); err != nil {
		a.appendLog("[shutdown] Failed to restore proxy settings: " + err.Error())
	}

	a.appendLog("[shutdown] SniShaper shutdown complete")
	if a.logFile != nil {
		_ = a.logFile.Close()
		a.logFile = nil
	}
}

func (a *App) setupFileLogger() {
	if a.logBuffer == nil {
		a.logBuffer = newRingLogWriter(5000)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stdout, a.logBuffer))
	a.appendLog("[startup] logger configured (in-memory only)")
}

func (a *App) appendLog(message string) {
	if a.logBuffer == nil {
		a.logBuffer = newRingLogWriter(5000)
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	if matched, _ := regexp.MatchString(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`, trimmed); matched {
		a.logBuffer.AppendLine(trimmed)
		return
	}
	a.logBuffer.AppendLine(time.Now().Format("2006/01/02 15:04:05.000000") + " " + trimmed)
}

func (a *App) GetRecentLogs(limit int) string {
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

	a.appendLog("[diag] GetRecentLogs fallback to file-read path")

	data, err := os.ReadFile(a.logPath)
	if err != nil {
		return ""
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return strings.Join(lines, "\n")
}

func (a *App) ClearLogs() error {
	if a.logBuffer != nil {
		a.logBuffer.Clear()
	}
	a.setupFileLogger()
	return nil
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) StartProxy() error {
	a.proxyOpMu.Lock()
	defer a.proxyOpMu.Unlock()

	a.appendLog("[action] StartProxy called")
	err := a.proxyServer.Start()
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "Only one usage of each socket address") || strings.Contains(msg, "bind: address already in use") {
			msg += " (端口被占用，请检查是否有旧版程序未关闭)"
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
	a.refreshTrayMenuLater(300*time.Millisecond, time.Second)
	a.emitFrontendState()
	a.appendLog("[action] StartProxy success")
	return nil
}

func (a *App) StopProxy() error {
	a.proxyOpMu.Lock()
	defer a.proxyOpMu.Unlock()

	a.appendLog("[action] StopProxy called")

	var errs []error

	if err := a.proxyServer.Stop(); err != nil {
		a.appendLog("[error] StopProxy failed: " + err.Error())
		errs = append(errs, err)
	}
	a.UpdateTrayMenu()

	if a.warpMgr != nil {
		if err := a.warpMgr.Stop(); err != nil {
			a.appendLog("[error] StopWarp during StopProxy failed: " + err.Error())
			errs = append(errs, err)
		}
	}

	if a.GetSystemProxyStatus().Enabled {
		if err := a.DisableSystemProxy(); err != nil {
			a.appendLog("[error] DisableSystemProxy during StopProxy failed: " + err.Error())
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		a.refreshTrayMenuLater(300 * time.Millisecond)
		a.emitFrontendState()
		return errors.Join(errs...)
	}
	a.refreshTrayMenuLater(300 * time.Millisecond)
	a.emitFrontendState()
	a.appendLog("[action] StopProxy success")
	return nil
}

func (a *App) IsProxyRunning() bool {
	return a.proxyServer.IsRunning()
}

func (a *App) GetStats() (int64, int64, int64) {
	return a.proxyServer.GetStats()
}

func (a *App) GetListenPort() int {
	addr := a.proxyServer.GetListenAddr()
	var port int
	fmt.Sscanf(addr, "127.0.0.1:%d", &port)
	return port
}

func (a *App) SetListenPort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d", port)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	err := a.proxyServer.SetListenAddr(addr)
	if err != nil {
		return err
	}
	a.ruleManager.SetListenPort(fmt.Sprintf("%d", port))
	return a.ruleManager.SaveConfig()
}

func (a *App) SetProxyMode(mode string) error {
	a.appendLog("[action] SetProxyMode: " + mode)
	err := a.proxyServer.SetMode(mode)
	if err != nil {
		a.appendLog("[error] SetProxyMode failed: " + err.Error())
	}
	return err
}

func (a *App) GetProxyMode() string {
	return a.proxyServer.GetMode()
}

func (a *App) GetCACertPath() string {
	if a.certManager != nil {
		return a.certManager.GetCACertPath()
	}
	return ""
}

type CAInstallStatus struct {
	Installed   bool
	Platform    string
	CertPath    string
	InstallHelp string
}

func (a *App) GetCAInstallStatus() CAInstallStatus {
	if a.certManager == nil {
		return CAInstallStatus{
			CertPath:    "",
			Platform:    "windows",
			InstallHelp: "证书管理器未初始化",
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

func (a *App) OpenCAFile() error {
	if a.certManager == nil {
		a.appendLog("[cert] OpenCAFile failed: cert manager not initialized")
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] OpenCAFile called")
	if err := a.certManager.OpenCAFile(); err != nil {
		a.appendLog("[cert] OpenCAFile failed: " + err.Error())
		return err
	}
	a.appendLog("[cert] OpenCAFile succeeded")
	return nil
}

func (a *App) OpenCertDir() error {
	if a.certManager == nil {
		a.appendLog("[cert] OpenCertDir failed: cert manager not initialized")
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] OpenCertDir called")
	if err := a.certManager.OpenCertDir(); err != nil {
		a.appendLog("[cert] OpenCertDir failed: " + err.Error())
		return err
	}
	a.appendLog("[cert] OpenCertDir succeeded")
	return nil
}

func (a *App) InstallCA() error {
	if a.certManager == nil {
		a.appendLog("[cert] InstallCA failed: cert manager not initialized")
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

func (a *App) GetCACertPEM() string {
	if a.certManager != nil {
		return a.certManager.GetCACertPEM()
	}
	return ""
}

func (a *App) RegenerateCert() error {
	if a.certManager == nil {
		a.appendLog("[cert] RegenerateCert failed: cert manager not initialized")
		return fmt.Errorf("cert manager not initialized")
	}
	a.appendLog("[cert] RegenerateCert called")
	if err := a.certManager.RegenerateCA(); err != nil {
		a.appendLog("[cert] RegenerateCert failed: " + err.Error())
		return err
	}
	a.appendLog("[cert] RegenerateCert succeeded")
	return nil
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

func (a *App) GetInstalledCerts() []cert.InstalledCert {
	if a.certManager == nil {
		a.appendLog("[cert] GetInstalledCerts failed: cert manager not initialized")
		return []cert.InstalledCert{}
	}
	a.appendLog("[cert] GetInstalledCerts called")
	certs, err := a.certManager.GetInstalledCertificates()
	if err != nil {
		a.appendLog("GetInstalledCertificates error: " + err.Error())
		return []cert.InstalledCert{}
	}
	a.appendLog(fmt.Sprintf("[cert] GetInstalledCerts succeeded: %d certs", len(certs)))
	return certs
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

func (a *App) GetECHProfiles() []proxy.ECHProfile {
	return a.ruleManager.GetECHProfiles()
}

func (a *App) UpsertECHProfile(p proxy.ECHProfile) error {
	return a.ruleManager.UpsertECHProfile(p)
}

func (a *App) DeleteECHProfile(id string) error {
	return a.ruleManager.DeleteECHProfile(id)
}

func (a *App) GetServerConfig() map[string]string {
	res := map[string]string{
		"host": "",
		"auth": "",
	}
	if a.ruleManager != nil {
		res["host"] = a.ruleManager.GetServerHost()
		res["auth"] = a.ruleManager.GetServerAuth()
	}
	return res
}

func (a *App) UpdateServerConfig(host, auth string) error {
	if a.ruleManager != nil {
		err := a.ruleManager.UpdateServerConfig(strings.TrimSpace(host), strings.TrimSpace(auth))
		if err == nil {
			a.appendLog(fmt.Sprintf("[INFO] Updated Server Worker settings, Host: %s", host))
		} else {
			a.appendLog(fmt.Sprintf("[ERROR] Failed to save Server settings: %v", err))
		}
		return err
	}
	return fmt.Errorf("RuleManager not initialized")
}

func (a *App) UpdateCloudflareConfig(cfg proxy.CloudflareConfig) error {
	// Get old config to check if AutoUpdate or Warp was toggled
	oldCfg := a.ruleManager.GetCloudflareConfig()

	err := a.ruleManager.UpdateCloudflareConfig(cfg)
	if err == nil {
		a.proxyServer.UpdateCloudflareConfig(cfg)
		// If toggled from false to true, trigger an update
		if cfg.AutoUpdate && !oldCfg.AutoUpdate {
			a.appendLog("[Cloudflare] Auto update enabled, triggering fetch...")
			go a.RefreshCloudflareIPPool()
		}

		// Handle Warp toggle
		if cfg.WarpEnabled && !oldCfg.WarpEnabled {
			a.appendLog("[Warp] Enabling Warp...")
			_ = a.warpMgr.SetEndpoint(cfg.WarpEndpoint)
			_ = a.warpMgr.Start()
		} else if !cfg.WarpEnabled && oldCfg.WarpEnabled {
			a.appendLog("[Warp] Disabling Warp...")
			_ = a.warpMgr.Stop()
		} else if cfg.WarpEnabled && cfg.WarpEndpoint != oldCfg.WarpEndpoint {
			a.appendLog(fmt.Sprintf("[Warp] Endpoint changed to %s, restarting...", cfg.WarpEndpoint))
			_ = a.warpMgr.Stop()
			_ = a.warpMgr.SetEndpoint(cfg.WarpEndpoint)
			_ = a.warpMgr.Start()
		}
		a.UpdateTrayMenu()
	}
	return err
}

func (a *App) StartWarp() error {
	a.warpOpMu.Lock()
	defer a.warpOpMu.Unlock()

	err := a.warpMgr.Start()
	a.UpdateTrayMenu()
	a.refreshTrayMenuLater(300*time.Millisecond, time.Second)
	a.emitFrontendState()
	return err
}

func (a *App) StopWarp() error {
	a.warpOpMu.Lock()
	defer a.warpOpMu.Unlock()

	err := a.warpMgr.Stop()
	a.UpdateTrayMenu()
	a.refreshTrayMenuLater(300*time.Millisecond)
	a.emitFrontendState()
	return err
}

func (a *App) GetWarpStatus() proxy.WarpStatus {
	return a.warpMgr.GetStatus()
}

func (a *App) RegisterWarp(deviceName string) (string, error) {
	return a.warpMgr.Register(deviceName)
}

func (a *App) EnrollWarp() (string, error) {
	return a.warpMgr.Enroll()
}

func (a *App) RefreshCloudflareIPPool() {
	cfg := a.ruleManager.GetCloudflareConfig()
	ips, err := proxy.FetchCloudflareIPs(cfg.APIKey)
	if err != nil {
		log.Printf("[Cloudflare] Failed to fetch preferred IPs: %v", err)
		a.appendLog("[error] Cloudflare 优选 IP 获取失败: " + err.Error())
		return
	}

	if len(ips) > 0 {
		log.Printf("[Cloudflare] Successfully fetched %d preferred IPs", len(ips))
		a.appendLog(fmt.Sprintf("[success] 成功获取 %d 个 Cloudflare 优选 IP", len(ips)))

		a.proxyServer.UpdateCloudflareIPPool(ips)
		// 持久化：同步到配置文件
		cfg.PreferredIPs = ips
		_ = a.ruleManager.UpdateCloudflareConfig(cfg)
	}
}

func (a *App) ForceFetchCloudflareIPs() error {
	cfg := a.ruleManager.GetCloudflareConfig()
	ips, err := proxy.FetchCloudflareIPs(cfg.APIKey)
	if err != nil {
		log.Printf("[Cloudflare] Failed to fetch preferred IPs: %v", err)
		a.appendLog("[error] 手动获取失败: " + err.Error())
		return err
	}

	if len(ips) > 0 {
		log.Printf("[Cloudflare] Successfully fetched %d preferred IPs", len(ips))
		a.appendLog(fmt.Sprintf("[success] 成功获取 %d 个 Cloudflare 优选 IP", len(ips)))
		a.proxyServer.UpdateCloudflareIPPool(ips)
		// 持久化：同步到配置文件
		cfg.PreferredIPs = ips
		_ = a.ruleManager.UpdateCloudflareConfig(cfg)
		// Trigger immediate health check to update stats
		a.proxyServer.TriggerCFHealthCheck()
	}
	return nil
}

func (a *App) GetCloudflareIPStats() []*proxy.IPStats {
	return a.proxyServer.GetAllCFIPsWithStats()
}

func (a *App) ExportConfig() (string, error) {
	return a.ruleManager.ExportConfig()
}

func (a *App) ImportConfig(content string) error {
	return a.ruleManager.ImportConfig(content)
}

func (a *App) ImportConfigWithSummary(content string) (proxy.ImportSummary, error) {
	return a.ruleManager.ImportConfigWithSummary(content)
}

type SystemProxyStatus struct {
	Enabled  bool
	Server   string
	Override string
}

type ProxyDiagnostics struct {
	Accepted      int64
	Requests      int64
	Connects      int64
	RecentIngress []string
}

func (a *App) TriggerCFHealthCheck() {
	a.proxyServer.TriggerCFHealthCheck()
	a.appendLog("[Cloudflare] 手动触发 IP 健康检查...")
}

func (a *App) RemoveInvalidCFIPs() int {
	count := a.proxyServer.RemoveInvalidCFIPs()
	a.appendLog(fmt.Sprintf("[Cloudflare] 已清理 %d 个失效 IP", count))
	return count
}

func (a *App) GetSystemProxyStatus() SystemProxyStatus {
	status := sysproxy.GetSystemProxyStatus()
	return SystemProxyStatus{
		Enabled:  status.Enabled,
		Server:   status.Server,
		Override: status.Override,
	}
}

func (a *App) EnableSystemProxy() error {
	a.appendLog("[action] EnableSystemProxy called")
	addr := a.proxyServer.GetListenAddr()
	var port int
	fmt.Sscanf(addr, "127.0.0.1:%d", &port)
	if port == 0 {
		port = 8080
	}
	if err := a.waitForProxyListen(addr, 1200*time.Millisecond); err != nil {
		a.appendLog("[error] EnableSystemProxy blocked: proxy not listening on " + addr)
		return fmt.Errorf("proxy is not listening on %s", addr)
	}
	err := sysproxy.EnableSystemProxy(port)
	if err != nil {
		a.appendLog("[error] EnableSystemProxy failed: " + err.Error())
		return err
	}
	a.UpdateTrayMenu()
	a.refreshTrayMenuLater(300 * time.Millisecond)
	a.emitFrontendState()
	a.appendLog(fmt.Sprintf("[action] EnableSystemProxy success: 127.0.0.1:%d", port))
	return nil
}

func (a *App) DisableSystemProxy() error {
	a.appendLog("[action] DisableSystemProxy called")
	err := sysproxy.DisableSystemProxy()
	if err != nil {
		a.appendLog("[error] DisableSystemProxy failed: " + err.Error())
		return err
	}
	a.UpdateTrayMenu()
	a.refreshTrayMenuLater(300 * time.Millisecond)
	a.emitFrontendState()
	a.appendLog("[action] DisableSystemProxy success")
	return nil
}

func (a *App) waitForProxyListen(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 250*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(80 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout")
	}
	return lastErr
}

func (a *App) GetProxyDiagnostics() map[string]interface{} {
	return map[string]interface{}{
		"ListenAddr": a.proxyServer.GetListenAddr(),
		"Status":     "OK",
	}
}

func (a *App) ProxySelfCheck() string {
	addr := a.proxyServer.GetListenAddr()
	a.appendLog("[diag] ProxySelfCheck started via " + addr)

	if !a.proxyServer.IsRunning() {
		msg := "[diag] ProxySelfCheck failed: proxy not running"
		a.appendLog(msg)
		return msg
	}

	proxyURL, err := url.Parse("http://" + addr)
	if err != nil {
		msg := "[diag] ProxySelfCheck failed: invalid proxy addr: " + err.Error()
		a.appendLog(msg)
		return msg
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: (&net.Dialer{
			Timeout:   6 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if err != nil {
		msg := "[diag] ProxySelfCheck failed: " + err.Error()
		a.appendLog(msg)
		return msg
	}

	resp, err := client.Do(req)
	if err != nil {
		msg := "[diag] ProxySelfCheck failed: " + err.Error()
		a.appendLog(msg)
		return msg
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2048))

	msg := fmt.Sprintf("[diag] ProxySelfCheck success: status=%d", resp.StatusCode)
	a.appendLog(msg)
	return msg
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
