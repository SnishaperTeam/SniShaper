package app

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"snishaper/pkg/sysproxy"
)

func (a *App) isManagedSystemProxy(status SystemProxyStatus) bool {
	if !status.Enabled {
		return false
	}
	expected := fmt.Sprintf("127.0.0.1:%s", a.GetListenPort())
	if strings.EqualFold(strings.TrimSpace(status.Server), expected) {
		return true
	}

	data, err := os.ReadFile(a.proxyMarkerPath)
	if err != nil {
		return false
	}
	var marker map[string]interface{}
	if err := json.Unmarshal(data, &marker); err != nil {
		return false
	}
	if v, ok := marker["expected_server"]; ok {
		if s, ok := v.(string); ok && strings.EqualFold(strings.TrimSpace(status.Server), s) {
			return true
		}
	}
	return false
}

func (a *App) saveManagedSystemProxyMarker(expected string) error {
	marker := map[string]interface{}{
		"expected_server": expected,
		"timestamp":       time.Now().Format(time.RFC3339),
	}
	data, err := json.Marshal(marker)
	if err != nil {
		return err
	}
	dir := filepath.Dir(a.proxyMarkerPath)
	_ = os.MkdirAll(dir, 0755)
	return os.WriteFile(a.proxyMarkerPath, data, 0644)
}

func (a *App) clearManagedSystemProxyMarker() error {
	_ = os.Remove(a.proxyMarkerPath)
	return nil
}

func (a *App) applySystemProxy(enabled bool, port int) error {
	return a.applySystemProxySync(enabled, port, false)
}

func (a *App) applySystemProxySync(enabled bool, port int, sync bool) error {
	a.systemProxyOpMu.Lock()
	status := a.GetSystemProxyStatus()
	expected := fmt.Sprintf("127.0.0.1:%d", port)

	if enabled {
		if status.Enabled && strings.EqualFold(strings.TrimSpace(status.Server), expected) {
			a.systemProxyOpMu.Unlock()
			if err := a.saveManagedSystemProxyMarker(expected); err != nil {
				a.appendLog("[warn] Failed to save managed system proxy marker: " + err.Error())
			}
			a.appendLog("[action] EnableSystemProxy skipped: already enabled")
			a.UpdateTrayMenu()
			a.emitFrontendState()
			return nil
		}
	} else {
		if !status.Enabled {
			a.systemProxyOpMu.Unlock()
			if err := a.clearManagedSystemProxyMarker(); err != nil {
				a.appendLog("[warn] Failed to clear managed system proxy marker: " + err.Error())
			}
			a.appendLog("[action] DisableSystemProxy skipped: already disabled")
			a.UpdateTrayMenu()
			a.emitFrontendState()
			return nil
		}
	}
	a.systemProxyOpMu.Unlock()

	a.appendLog(fmt.Sprintf("[action] %s system proxy %s...", map[bool]string{true: "Enabling", false: "Disabling"}[enabled], map[bool]string{true: "synchronously", false: "asynchronously"}[sync]))

	work := func() error {
		var err error
		if enabled {
			err = sysproxy.EnableSystemProxy(port)
		} else {
			err = sysproxy.DisableSystemProxy()
		}
		if err != nil {
			return err
		}
		if enabled {
			if err := a.saveManagedSystemProxyMarker(expected); err != nil {
				a.appendLog("[warn] Failed to save managed system proxy marker: " + err.Error())
			}
			a.appendLog(fmt.Sprintf("[action] EnableSystemProxy success: 127.0.0.1:%d", port))
		} else {
			if err := a.clearManagedSystemProxyMarker(); err != nil {
				a.appendLog("[warn] Failed to clear managed system proxy marker: " + err.Error())
			}
			a.appendLog("[action] DisableSystemProxy success")
		}
		return nil
	}

	if sync {
		a.systemProxyOpMu.Lock()
		err := work()
		a.systemProxyOpMu.Unlock()
		if err != nil {
			a.appendLog("[error] Sync ApplySystemProxy failed: " + err.Error())
		}
		a.UpdateTrayMenu()
		a.refreshTrayMenuLater(300 * time.Millisecond)
		a.emitFrontendState()
		return err
	}

	go func() {
		a.systemProxyOpMu.Lock()
		defer a.systemProxyOpMu.Unlock()
		if err := work(); err != nil {
			a.appendLog("[error] Async ApplySystemProxy failed: " + err.Error())
		}
		a.UpdateTrayMenu()
		a.refreshTrayMenuLater(300 * time.Millisecond)
		a.emitFrontendState()
	}()

	return nil
}

func (a *App) EnableSystemProxy() error {
	a.appendLog("[action] EnableSystemProxy called")

	if !a.IsProxyRunning() {
		a.appendLog("[action] Proxy not running, starting proxy before enabling system proxy...")
		if err := a.StartProxy(); err != nil {
			a.appendLog("[error] EnableSystemProxy failed to auto-start proxy: " + err.Error())
			return err
		}
	}

	addr := a.proxyServer.GetListenAddr()
	var port int
	fmt.Sscanf(addr, "127.0.0.1:%d", &port)
	if port == 0 {
		port = 8080
	}
	if err := a.waitForProxyListen(addr, 500*time.Millisecond); err != nil {
		a.appendLog("[warn] EnableSystemProxy probe timeout (expected if already running): " + err.Error())
	}
	err := a.applySystemProxy(true, port)
	if err != nil {
		a.appendLog("[error] EnableSystemProxy failed: " + err.Error())
		return err
	}
	a.appendLog(fmt.Sprintf("[action] EnableSystemProxy requested (async): 127.0.0.1:%d", port))
	return nil
}

func (a *App) DisableSystemProxy() error {
	a.appendLog("[action] DisableSystemProxy called")
	err := a.applySystemProxy(false, 0)
	if err != nil {
		a.appendLog("[error] DisableSystemProxy failed: " + err.Error())
		return err
	}
	a.appendLog("[action] DisableSystemProxy requested (async)")
	return nil
}

func (a *App) GetSystemProxyStatus() SystemProxyStatus {
	status := sysproxy.GetSystemProxyStatus()
	return SystemProxyStatus{
		Enabled:  status.Enabled,
		Server:   status.Server,
		Override: status.Override,
	}
}

func (a *App) GetAutoStart() bool {
	return a.ruleManager.GetAutoStart()
}

func (a *App) SetAutoStart(enabled bool) error {
	a.appendLog(fmt.Sprintf("[action] SetAutoStart called: %v", enabled))
	err := a.ruleManager.SetAutoStart(enabled)
	if err != nil {
		a.appendLog(fmt.Sprintf("[error] SetAutoStart failed: %v", err))
		return err
	}
	var command string
	if enabled {
		execPath, err := os.Executable()
		if err == nil {
			command = buildAutoStartCommand(execPath)
		}
	}
	if err := setAutoStartEnabled(enabled, command); err != nil {
		a.appendLog(fmt.Sprintf("[error] SetAutoStart register failed: %v", err))
		return err
	}
	a.UpdateTrayMenu()
	return nil
}

func (a *App) syncAutoStartRegistration() error {
	if !a.GetAutoStart() {
		return setAutoStartEnabled(false, "")
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path")
	}
	command := buildAutoStartCommand(execPath)
	return setAutoStartEnabled(true, command)
}

func (a *App) GetShowMainWindowOnAutoStart() bool {
	return a.ruleManager.GetShowMainWindowOnAutoStart()
}

func (a *App) SetShowMainWindowOnAutoStart(enabled bool) error {
	a.appendLog(fmt.Sprintf("[action] SetShowMainWindowOnAutoStart called: %v", enabled))
	return a.ruleManager.SetShowMainWindowOnAutoStart(enabled)
}

func (a *App) GetAutoEnableProxyOnAutoStart() bool {
	return a.ruleManager.GetAutoEnableProxyOnAutoStart()
}

func (a *App) SetAutoEnableProxyOnAutoStart(enabled bool) error {
	a.appendLog(fmt.Sprintf("[action] SetAutoEnableProxyOnAutoStart called: %v", enabled))
	return a.ruleManager.SetAutoEnableProxyOnAutoStart(enabled)
}

// ForceCleanup is a last-resort synchronous cleanup for crash/force-exit paths.
// It stops TUN, core, proxy, and system proxy without checking state.
func (a *App) ForceCleanup() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ForceCleanup] panic: %v", r)
		}
	}()
	a.systemProxyOpMu.Lock()
	defer a.systemProxyOpMu.Unlock()
	if a.core != nil {
		_ = a.core.StopTUN()
		a.core.ShutdownIfRunning()
	}
	if a.proxyServer != nil {
		_ = a.proxyServer.Stop()
	}
	if err := sysproxy.DisableSystemProxy(); err != nil {
		log.Printf("[ForceCleanup] sysproxy disable: %v", err)
	}
}

func (a *App) RevealMainWindow() {
	if a.mainWindow != nil {
		a.mainWindow.Show()
		a.mainWindow.Focus()
	}
}

func (a *App) GetCloseToTray() bool {
	return a.ruleManager.GetCloseToTray()
}

func (a *App) SetCloseToTray(enabled bool) error {
	a.appendLog(fmt.Sprintf("[action] SetCloseToTray: %v", enabled))
	return a.ruleManager.SetCloseToTray(enabled)
}

func (a *App) WindowMinimise() {
	if a.mainWindow != nil {
		a.mainWindow.Minimise()
	}
}

func (a *App) WindowToggleMaximise() {
	if a.mainWindow != nil {
		a.mainWindow.ToggleMaximise()
	}
}

func (a *App) WindowClose() {
	a.QuitApp()
}

func (a *App) HandleWindowClose() {
	if a.GetCloseToTray() && !a.shouldQuit && a.mainWindow != nil {
		a.mainWindow.Hide()
		return
	}
	a.QuitApp()
}
