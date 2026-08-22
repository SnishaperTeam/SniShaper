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
)

// UIAdapter receives frontend-bound events in headless (CLI) mode. The GUI
// binary wires these to the webview window; the CLI binary leaves it nil.
type UIAdapter interface {
	Emit(event string, payload map[string]interface{})
}

// SetUIAdapter wires an optional event sink (window/tray in the GUI build).
func (a *App) SetUIAdapter(ui UIAdapter) { a.ui = ui }

// SetCLIMode marks the instance as headless; startup skips OS-level
// autostart registration that belongs to the desktop app.
func (a *App) SetCLIMode(enabled bool) { a.cliMode = enabled }

// SetSilentStdout suppresses stdout logging (used by the TUI so raw log
// lines never corrupt the terminal screen; logs still go to the ring
// buffer and log file).
func (a *App) SetSilentStdout(enabled bool) { a.silentStdout = enabled }

type App struct {
	ui                  UIAdapter
	cliMode             bool
	silentStdout        bool
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
	proxyOpMu           sync.Mutex // lock order: proxyOpMu → systemProxyOpMu (never reverse)
	systemProxyOpMu     sync.Mutex
	wg                  sync.WaitGroup
	ctx                 context.Context
	cancel              context.CancelFunc
	launchedAtStartup   bool
	autoProxyAtStartup  bool
	core                *core.CoreClient
	tunRestoreSysProxy  bool
	pendingUpdateMu     sync.Mutex
	pendingUpdatePath   string
	downloadConcurrency int
	downloadChunkSize   int64
}

// ShouldQuit returns whether the app should quit.
func (a *App) ShouldQuit() bool { return a.shouldQuit }

// RunSafeAsync runs a function safely in a goroutine.
func (a *App) RunSafeAsync(taskName string, fn func()) { a.runSafeAsync(taskName, fn) }

// emit delivers an event to the optional UI adapter.
func (a *App) emit(event string, payload map[string]interface{}) {
	if a.ui != nil {
		a.ui.Emit(event, payload)
	}
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

func (a *App) setupFileLogger() {
	if a.logBuffer == nil {
		a.logBuffer = common.NewRingLogWriter(500)
	}
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if a.silentStdout {
		log.SetOutput(&gatedLogWriter{app: a})
	} else {
		log.SetOutput(io.MultiWriter(&gatedLogWriter{app: a}, os.Stdout))
	}
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

func (a *App) Startup() error {
	a.startup()
	return nil
}

func (a *App) Shutdown() error {
	a.shutdown()
	return nil
}

func (a *App) startup() {
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
	if !a.cliMode {
		if err := a.syncAutoStartRegistration(); err != nil {
			a.appendLog("[startup] Auto-start sync check failed: " + err.Error())
		}
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
// UI adapter (app:route), since the core RPC only buffers them for polling.
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
			if a.core == nil {
				continue
			}
			events := a.core.GetRouteEvents()
			if len(events) == 0 {
				continue
			}
			for _, e := range events {
				if a.shouldQuit {
					return
				}
				a.emit("app:route", map[string]interface{}{
					"domain": e.Domain,
					"mode":   e.Mode,
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
}

func (a *App) UpdateTrayMenu() {
}

func (a *App) emitFrontendState() {
	if a.shouldQuit {
		return
	}
	a.UpdateTrayMenu()
	tunStatus := a.GetTUNStatus()
	a.emit("app:state_changed", map[string]interface{}{
		"proxyRunning":      a.IsProxyRunning(),
		"systemProxyActive": a.GetSystemProxyStatus().Enabled,
		"proxyMode":         a.GetProxyMode(),
		"tunRunning":        tunStatus.Running,
		"tunMessage":        tunStatus.Message,
		"ipv6Available":     a.checkIPv6Available(),
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
