package app

import (
	"context"
	"io"
	"log"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"snishaper/common"
	"snishaper/core"
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
	logBuffer         *common.RingLogWriter
	logCaptureMu      sync.RWMutex
	logCaptureEnabled bool
	shouldQuit        bool
	systemTray        *application.SystemTray
	trayMenuV3        *application.Menu
	proxyItemV3       *application.MenuItem
	systemProxyItemV3 *application.MenuItem
	proxyOpMu         sync.Mutex // lock order: proxyOpMu → systemProxyOpMu (never reverse)
	systemProxyOpMu   sync.Mutex
	wg                sync.WaitGroup
	ctx               context.Context
	cancel            context.CancelFunc
	launchedAtStartup bool
	core              *core.CoreClient
	tunRestoreSysProxy bool
}

// SetWailsApp sets the wails application instance.
func (a *App) SetWailsApp(w *application.App) { a.wailsApp = w }

// SetMainWindow sets the main window reference.
func (a *App) SetMainWindow(w *application.WebviewWindow) { a.mainWindow = w }

// SetSystemTray sets the system tray reference.
func (a *App) SetSystemTray(t *application.SystemTray) { a.systemTray = t }

// SetTrayMenu sets the tray menu reference.
func (a *App) SetTrayMenu(m *application.Menu) { a.trayMenuV3 = m }

// SetProxyMenuItem sets the proxy menu item reference.
func (a *App) SetProxyMenuItem(i *application.MenuItem) { a.proxyItemV3 = i }

// SetSystemProxyMenuItem sets the system proxy menu item reference.
func (a *App) SetSystemProxyMenuItem(i *application.MenuItem) { a.systemProxyItemV3 = i }

// ShouldQuit returns whether the app should quit.
func (a *App) ShouldQuit() bool { return a.shouldQuit }

// RunSafeAsync runs a function safely in a goroutine.
func (a *App) RunSafeAsync(taskName string, fn func()) { a.runSafeAsync(taskName, fn) }

type gatedLogWriter struct {
	app *App
}

func (g *gatedLogWriter) Write(p []byte) (n int, err error) {
	if g.app != nil {
		g.app.appendLog(string(p))
	}
	return len(p), nil
}

func (a *App) setupFileLogger() {
	if a.logBuffer == nil {
		a.logBuffer = common.NewRingLogWriter(500)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(&gatedLogWriter{app: a}, os.Stdout))
}

func (a *App) appendLog(message string) {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return
	}

	var formatted string
	if matched, _ := regexp.MatchString(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`, trimmed); matched {
		formatted = trimmed
	} else {
		formatted = time.Now().Format("2006/01/02 15:04:05.000000") + " " + trimmed
	}

	if a.logBuffer == nil {
		a.logBuffer = common.NewRingLogWriter(500)
	}
	a.logBuffer.Write([]byte(formatted + "\n"))
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
		if err := a.core.EnsureRunning(); err != nil {
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

	// 1. Stop TUN first (depends on core/proxy running)
	if a.core != nil {
		tunStatus := a.core.GetTUNStatus()
		if tunStatus.Running {
			a.appendLog("[shutdown] Stopping TUN...")
			if err := a.core.StopTUN(); err != nil {
				errs = append(errs, "StopTUN: "+err.Error())
			}
		}
	}

	// 2. Disable system proxy synchronously
	status := a.GetSystemProxyStatus()
	if status.Enabled {
		a.appendLog("[shutdown] Disabling system proxy...")
		if err := a.applySystemProxySync(false, 0, true); err != nil {
			errs = append(errs, "SystemProxy: "+err.Error())
		}
	}

	// 3. Stop proxy server
	if a.IsProxyRunning() {
		a.appendLog("[shutdown] Stopping proxy...")
		if err := a.proxyServer.Stop(); err != nil {
			errs = append(errs, "StopProxy: "+err.Error())
		}
	}

	// 4. Shut down core process
	if a.core != nil {
		a.appendLog("[shutdown] Shutting down core...")
		a.core.ShutdownIfRunning()
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
	if a.shouldQuit {
		return
	}
	a.UpdateTrayMenu()
	if a.mainWindow == nil {
		return
	}
	application.InvokeAsync(func() {
		if a.mainWindow == nil || a.shouldQuit {
			return
		}
		tunStatus := a.GetTUNStatus()
		a.mainWindow.EmitEvent("app:state_changed", map[string]interface{}{
			"proxyRunning":      a.IsProxyRunning(),
			"systemProxyActive": a.GetSystemProxyStatus().Enabled,
			"proxyMode":         a.GetProxyMode(),
			"tunRunning":        tunStatus.Running,
			"tunMessage":        tunStatus.Message,
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
// HasLaunchArg checks if the given argument was passed to the application.
func HasLaunchArg(arg string) bool {
	for _, a := range os.Args {
		if strings.EqualFold(a, arg) {
			return true
		}
	}
	return false
}

func (a *App) ShouldStartHidden() bool {
	return a.launchedAtStartup && !a.GetShowMainWindowOnAutoStart()
}

func (a *App) ShouldAutoEnableProxyOnAutoStart() bool {
	return a.launchedAtStartup && a.GetAutoEnableProxyOnAutoStart()
}
