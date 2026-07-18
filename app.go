package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"snishaper/evolution"
	"snishaper/pkg/certmanager"
	"snishaper/proxy"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	wailsApp          *application.App
	mainWindow        *application.WebviewWindow
	proxyServer       *proxy.ProxyServer
	certManager       *certmanager.CertManager
	ruleManager       *proxy.RuleManager
	evolutionTester   *evolution.Tester
	certPath          string
	proxyMarkerPath   string
	logBuffer         *ringLogWriter
	logCaptureMu      sync.RWMutex
	logCaptureEnabled bool
	shouldQuit        bool
	systemTray        *application.SystemTray
	trayMenuV3        *application.Menu
	proxyItemV3       *application.MenuItem
	systemProxyItemV3 *application.MenuItem
	proxyOpMu         sync.Mutex
	systemProxyOpMu   sync.Mutex
	wg                sync.WaitGroup
	ctx               context.Context
	cancel            context.CancelFunc
	launchedAtStartup bool
	core              *coreClient
}

type gatedLogWriter struct {
	app *App
}

func (g *gatedLogWriter) Write(p []byte) (n int, err error) {
	if g.app != nil {
		g.app.appendLog(string(p))
	}
	return len(p), nil
}

type ringLogWriter struct {
	mu       sync.Mutex
	lines    []string
	capacity int
	size     int
	start    int
}

func newRingLogWriter(capacity int) *ringLogWriter {
	return &ringLogWriter{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

func (w *ringLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	msg := string(p)
	lines := strings.Split(msg, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if w.size < w.capacity {
			w.lines[w.size] = trimmed
			w.size++
		} else {
			w.lines[w.start] = trimmed
			w.start = (w.start + 1) % w.capacity
		}
	}
	return len(p), nil
}

func (w *ringLogWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.start = 0
	w.size = 0
}

func (w *ringLogWriter) Snapshot(limit int) []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if limit <= 0 || limit > w.size {
		limit = w.size
	}

	result := make([]string, limit)
	if w.size < w.capacity {
		copy(result, w.lines[w.size-limit:w.size])
	} else {
		for i := 0; i < limit; i++ {
			idx := (w.start + w.size - limit + i) % w.capacity
			result[i] = w.lines[idx]
		}
	}
	return result
}

func (a *App) setupFileLogger() {
	if a.logBuffer == nil {
		a.logBuffer = newRingLogWriter(500)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(&gatedLogWriter{app: a}, os.Stdout))
}

func (a *App) appendLog(message string) {
	if !a.IsLogCaptureEnabled() {
		return
	}
	if a.logBuffer == nil {
		a.logBuffer = newRingLogWriter(500)
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}
	if !strings.HasSuffix(trimmed, "\n") {
		trimmed += "\n"
	}
	if matched, _ := regexp.MatchString(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`, trimmed); matched {
		a.logBuffer.Write([]byte(trimmed))
		return
	}
	formatted := time.Now().Format("2006/01/02 15:04:05.000000") + " " + trimmed
	a.logBuffer.Write([]byte(formatted))
}

func (a *App) IsLogCaptureEnabled() bool {
	a.logCaptureMu.RLock()
	defer a.logCaptureMu.RUnlock()
	return a.logCaptureEnabled
}

func (a *App) StartLogCapture() error {
	a.logCaptureMu.Lock()
	a.logCaptureEnabled = true
	a.logCaptureMu.Unlock()
	if a.core != nil {
		_ = a.core.StartLogCapture()
	}
	a.appendLog("[action] StartLogCapture")
	return nil
}

func (a *App) StopLogCapture() error {
	a.logCaptureMu.Lock()
	a.logCaptureEnabled = false
	a.logCaptureMu.Unlock()
	if a.core != nil {
		_ = a.core.StopLogCapture()
	}
	return nil
}

func (a *App) ServiceStartup(ctx *application.Context) {
	a.startupV3()
}

func (a *App) ServiceShutdown() {
	a.shutdown()
}

func (a *App) startupV3() {
	a.setupFileLogger()
	log.Printf("[startup] SniShaper startup hook entered")
	a.appendLog("[startup] in-memory log channel ready")

	var err error
	a.certManager, err = certmanager.InitCertManager(a.certPath)
	if err != nil {
		a.appendLog("[startup] Failed to init cert manager: " + err.Error())
	} else {
		a.appendLog("[startup] Cert manager initialized: " + a.certPath)
	}

	if err := a.ruleManager.LoadConfig(); err != nil {
		a.appendLog("[startup] Failed to load config: " + err.Error())
	}
	if err := a.syncAutoStartRegistration(); err != nil {
		a.appendLog("[startup] Auto-start sync check failed: " + err.Error())
	}

	if a.core != nil {
		if err := a.core.ensureRunning(); err != nil {
			a.appendLog("[startup] WARNING: Core service client start failed: " + err.Error())
		} else {
			a.appendLog("[startup] Core process synchronized successfully")
		}
	}

	if a.ShouldAutoEnableProxyOnAutoStart() {
		a.appendLog("[startup] AutoStart: Auto-enabling proxy as configured")
		a.runSafeAsync("startup proxy sync", func() {
			if err := a.StartProxy(); err != nil {
				a.appendLog("[startup] AutoStart StartProxy failed: " + err.Error())
				return
			}
			if err := a.EnableSystemProxy(); err != nil {
				a.appendLog("[startup] AutoStart EnableSystemProxy failed: " + err.Error())
			}
		})
	}
}

func (a *App) shutdown() {
	a.appendLog("[shutdown] SniShaper shutdown hook entered")
	a.cancel()

	var errs []string

	// Shut down the core process if it's managing the proxy
	if a.core != nil {
		a.core.shutdownIfRunning()
	}

	status := a.GetSystemProxyStatus()
	if status.Enabled && a.isManagedSystemProxy(status) {
		a.appendLog("[shutdown] Restoring original system proxy...")
		if err := a.applySystemProxy(false, 0); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if a.IsProxyRunning() {
		if err := a.proxyServer.Stop(); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		log.Printf("[shutdown] Shutdown completed with errors: %s", strings.Join(errs, "; "))
	} else {
		log.Printf("[shutdown] Shutdown completed cleanly")
	}

	a.wg.Wait()
}

func (a *App) runSafeAsync(taskName string, fn func()) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[App] panic in async task %s: %v\n%s", taskName, r, string(debug.Stack()))
			}
		}()
		fn()
	}()
}

func (a *App) refreshTrayMenuLater(delays ...time.Duration) {
	go func() {
		for _, d := range delays {
			time.Sleep(d)
			a.UpdateTrayMenu()
		}
	}()
}

func (a *App) UpdateTrayMenu() {
	if a.systemTray == nil || a.trayMenuV3 == nil {
		return
	}

	running := a.IsProxyRunning()
	proxyLabel := "代理: 关"
	if running {
		proxyLabel = "代理: 开"
	}
	if a.proxyItemV3 != nil {
		a.proxyItemV3.SetLabel(proxyLabel)
		a.proxyItemV3.SetChecked(running)
	}

	status := a.GetSystemProxyStatus()
	sysProxyLabel := "系统代理: 关"
	if status.Enabled {
		sysProxyLabel = "系统代理: 开"
	}
	if a.systemProxyItemV3 != nil {
		a.systemProxyItemV3.SetLabel(sysProxyLabel)
	}
}

func (a *App) emitFrontendState() {
	if a.mainWindow == nil {
		return
	}
	application.InvokeAsync(func() {
		if a.mainWindow == nil {
			return
		}
		a.mainWindow.EmitEvent("app:state_changed", map[string]interface{}{
			"proxyRunning":      a.IsProxyRunning(),
			"systemProxyActive": a.GetSystemProxyStatus().Enabled,
			"proxyMode":         a.GetProxyMode(),
		})
	})
}

// Struct declarations required by frontend / main package
type CAInstallStatus struct {
	Installed   bool   `json:"Installed"`
	Platform    string `json:"Platform"`
	CertPath    string `json:"CertPath"`
	InstallHelp string `json:"InstallHelp"`
}

type SystemProxyStatus struct {
	Enabled  bool   `json:"Enabled"`
	Server   string `json:"Server"`
	Override string `json:"Override"`
}

type CheckUpdateResult struct {
	HasUpdate     bool   `json:"has_update"`
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
	Message       string `json:"message"`
	ErrorDetail   string `json:"error_detail"`
}

type DNSTestResult struct {
	Success bool     `json:"success"`
	IPs     []string `json:"ips,omitempty"`
	Latency string   `json:"latency,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// Helpers
func hasLaunchArg(arg string) bool {
	for _, a := range os.Args {
		if strings.EqualFold(a, arg) {
			return true
		}
	}
	return false
}

func resolveRuntimeFile(execDir, relPath string) string {
	absPath := filepath.Join(execDir, relPath)
	if _, err := os.Stat(absPath); err == nil {
		return absPath
	}
	fallback := filepath.Join(".", relPath)
	return fallback
}

func (a *App) ShouldStartHidden() bool {
	return a.launchedAtStartup && !a.GetShowMainWindowOnAutoStart()
}

func (a *App) ShouldAutoEnableProxyOnAutoStart() bool {
	return a.launchedAtStartup && a.GetAutoEnableProxyOnAutoStart()
}
