package app

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
	wailsApp            *application.App
	mainWindow          *application.WebviewWindow
	proxyServer         *proxy.ProxyServer
	certManager         *certmanager.CertManager
	ruleManager         *proxy.RuleManager
	evolutionTester     *evolution.Tester
	evolutionTesterMu   sync.Mutex
	certPath            string
	proxyMarkerPath     string
	logBuffer           *common.RingLogWriter
	logDir              string
	logFilePath         string
	logFile             *os.File
	logFileMu           sync.Mutex
	logCaptureMu        sync.RWMutex
	logCaptureEnabled   bool
	shouldQuit          bool
	systemTray          *application.SystemTray
	trayMenuV3          *application.Menu
	proxyItemV3         *application.MenuItem
	systemProxyItemV3   *application.MenuItem
	proxyOpMu           sync.Mutex // lock order: proxyOpMu → systemProxyOpMu (never reverse)
	systemProxyOpMu     sync.Mutex
	wg                  sync.WaitGroup
	ctx                 context.Context
	cancel              context.CancelFunc
	launchedAtStartup   bool
	autoProxyAtStartup  bool
	core                *core.CoreClient
	tunRestoreSysProxy  bool
	pendingShow         bool
	pendingUpdateMu     sync.Mutex
	pendingUpdatePath   string
	downloadConcurrency int
	downloadChunkSize   int64
}

// SetWailsApp sets the wails application instance.
func (a *App) SetWailsApp(w *application.App) { a.wailsApp = w }

// SetMainWindow sets the main window reference.
func (a *App) SetMainWindow(w *application.WebviewWindow) {
	if w != nil {
		a.mainWindow = w
		if a.pendingShow {
			a.mainWindow.Show()
			a.mainWindow.Focus()
			a.pendingShow = false
		}
	} else {
		a.mainWindow = w
	}
}

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
	a.openLogFile()
}

// openLogFile creates a new timestamped log file for this run in
// <execDir>/log/. One file per run: opened at startup, closed at shutdown.
func (a *App) openLogFile() {
	if a.logDir == "" {
		if ep, err := os.Executable(); err == nil {
			a.logDir = filepath.Join(filepath.Dir(ep), "log")
		} else {
			return
		}
	}
	if err := os.MkdirAll(a.logDir, 0755); err != nil {
		a.appendLog("[warn] Failed to create log dir: " + err.Error())
		return
	}
	name := time.Now().Format("2006-01-02_15-04-05") + ".log"
	f, err := os.OpenFile(filepath.Join(a.logDir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		a.appendLog("[warn] Failed to open log file: " + err.Error())
		return
	}
	a.logFileMu.Lock()
	a.logFile = f
	a.logFilePath = filepath.Join(a.logDir, name)
	a.logFileMu.Unlock()
	a.appendLog("[startup] Log file: " + a.logFilePath)
}

func (a *App) closeLogFile() {
	a.logFileMu.Lock()
	defer a.logFileMu.Unlock()
	if a.logFile != nil {
		_ = a.logFile.Close()
		a.logFile = nil
	}
}

func (a *App) writeLogFile(s string) {
	a.logFileMu.Lock()
	defer a.logFileMu.Unlock()
	if a.logFile != nil {
		_, _ = a.logFile.WriteString(s)
	}
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
	a.writeLogFile(formatted + "\n")
}

// startLogFileMirror continuously appends the core process's logs into the
// current log file, so the persisted log captures both app and core activity.
// ponytail: anchor-line dedup; safe while the core ring (5000 lines) still
// holds the last mirrored line between 2s polls.
func (a *App) startLogFileMirror() {
	a.runSafeAsync("core log file mirror", func() {
		anchor := ""
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
			}
			if a.core == nil || a.logFile == nil {
				continue
			}
			logs := strings.TrimSpace(a.core.GetRecentLogs(200))
			if logs == "" {
				continue
			}
			lines := strings.Split(logs, "\n")
			if anchor != "" {
				idx := -1
				for i, l := range lines {
					if strings.TrimSpace(l) == anchor {
						idx = i
					}
				}
				if idx >= 0 {
					lines = lines[idx+1:]
				}
			}
			if len(lines) == 0 {
				continue
			}
			anchor = strings.TrimSpace(lines[len(lines)-1])
			a.writeLogFile(strings.Join(lines, "\n") + "\n")
		}
	})
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

	// Boot diagnostics: helps root-cause autostart failures (args missing,
	// wrong CWD, not elevated) without attaching a debugger.
	if execPath, err := os.Executable(); err == nil {
		cwd, _ := os.Getwd()
		a.appendLog(fmt.Sprintf("[startup] args=%v cwd=%q execDir=%q elevated=%v startupFlag=%v autoProxyFlag=%v autoEnableCfg=%v",
			os.Args, cwd, filepath.Dir(execPath), core.IsProcessElevated(),
			a.launchedAtStartup, a.autoProxyAtStartup, a.GetAutoEnableProxyOnAutoStart()))
	}

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
			a.autoEnableProxyAtStartup()
		})
	}

	a.startIPv6Monitor()
	a.RefreshIPv6Check()
	a.startRouteEventsPoller()
	a.startLogFileMirror()
}

// autoEnableProxyAtStartup starts the proxy (and system proxy) with backoff
// retries so transient boot-time failures — network stack or Wintun driver not
// ready, port still held by a previous instance, core RPC race — don't leave
// autostart silently dead. Manual clicks work later because the system has settled.
// ponytail: 4 attempts @ 0/2s/4s/8s; make counts configurable if users ask.
func (a *App) autoEnableProxyAtStartup() {
	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}
	for attempt := 0; ; attempt++ {
		err := a.StartProxy()
		if err == nil {
			if sysErr := a.EnableSystemProxy(); sysErr != nil {
				a.appendLog("[startup] AutoStart EnableSystemProxy failed: " + sysErr.Error())
			}
			a.appendLog(fmt.Sprintf("[startup] AutoStart proxy enabled (attempt %d)", attempt+1))
			return
		}
		a.appendLog(fmt.Sprintf("[startup] AutoStart StartProxy attempt %d failed: %v", attempt+1, err))
		if attempt >= len(delays) {
			a.appendLog("[startup] AutoStart proxy enable failed after all retries; proxy left off")
			if a.systemTray != nil {
				a.systemTray.SetTooltip("SniShaper: 开机自启代理启动失败，请手动开启代理")
			}
			return
		}
		select {
		case <-time.After(delays[attempt]):
		case <-a.ctx.Done():
			return
		}
	}
}

// startRouteEventsPoller forwards route events from the core process to the
// frontend (app:route), since the core RPC only buffers them for polling.
func (a *App) startRouteEventsPoller() {
	a.runSafeAsync("route events poller", func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
			}
			if a.core == nil || a.mainWindow == nil {
				continue
			}
			events := a.core.GetRouteEvents()
			if len(events) == 0 {
				continue
			}
			for _, e := range events {
				application.InvokeAsync(func() {
					if a.mainWindow == nil || a.shouldQuit {
						return
					}
					a.mainWindow.EmitEvent("app:route", map[string]interface{}{
						"domain": e.Domain,
						"mode":   e.Mode,
					})
				})
			}
		}
	})
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
	a.closeLogFile()
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
			"ipv6Available":     a.checkIPv6Available(),
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

type ReleaseAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
	Kind        string `json:"kind"`
}

type DownloadResult struct {
	LocalPath string `json:"local_path"`
	Size      int64  `json:"size"`
}

type CheckUpdateResult struct {
	HasUpdate     bool           `json:"has_update"`
	LatestVersion string         `json:"latest_version"`
	Channel       string         `json:"channel"`
	ReleaseName   string         `json:"release_name"`
	ReleaseNotes  string         `json:"release_notes"`
	Assets        []ReleaseAsset `json:"assets"`
	DownloadURL   string         `json:"download_url"`
	Message       string         `json:"message"`
	ErrorDetail   string         `json:"error_detail"`
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
	return (a.launchedAtStartup || a.autoProxyAtStartup) && a.GetAutoEnableProxyOnAutoStart()
}
