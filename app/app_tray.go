//go:build !headless

package app

import (
	"log"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Background: Wails v3 beta.26 only re-adds the tray icon when the shell
// broadcasts TaskbarCreated (Explorer restart). Sleep/resume, session
// lock/unlock, and silent shell drops never trigger it, so the app requests a
// reshow at those moments.
//
// Recovery must go through Wails' internal reshowSystrays()
// (v3/pkg/application/application_windows.go), which calls systray.show() on
// every registered tray. show() issues NIM_ADD + NIM_SETVERSION against the
// existing HWND/UID, so it is idempotent: if the shell still holds the icon it
// is a no-op refresh; if the shell dropped it, the original slot is restored.
//
// Never rebuild via SystemTray.New(): Wails' systrayMap is keyed by HWND, and
// each New() allocates a fresh HWND plus a fresh UID, which the Shell treats as
// a second, independent icon. That is exactly the "double icon after wake,
// one disappears when the mouse passes over it" symptom.
//
// requestTrayReshow is implemented in app_tray_reshow_windows.go; on other
// builds it is a no-op in app_tray_reshow_other.go.

const (
	trayRebuildMinInterval = 3 * time.Second
	trayRebuildPeriod      = 30 * time.Minute
)

type trayHolder struct {
	mu         sync.Mutex
	app        *application.App
	icon       []byte
	current    *application.SystemTray
	lastReshow time.Time
}

var tray trayHolder

// BuildSystemTray creates the tray icon, starts the periodic refresh loop,
// and wires up the two recovery hooks that request a reshow after the shell
// drops it: system resume (SystemDidWake) and user session unlock
// (WTS_SESSION_UNLOCK, Windows only). It must be called before the runtime
// starts, so the tray is registered while the app is still starting up.
func (a *App) BuildSystemTray(wailsApp *application.App, icon []byte) {
	tray.mu.Lock()
	tray.app = wailsApp
	tray.icon = icon
	tray.mu.Unlock()

	a.startSessionWatcher()
	a.buildSystemTray()
	go a.trayRefreshLoop()

	wailsApp.Event.OnApplicationEvent(events.Common.SystemDidWake, func(_ *application.ApplicationEvent) {
		if isSessionLocked() {
			log.Printf("[tray] system resumed while locked, deferring rebuild to session unlock")
			return
		}
		log.Printf("[tray] system resumed (not locked), requesting tray reshow")
		a.RebuildSystemTray()
	})
}

func (a *App) buildSystemTray() {
	tray.mu.Lock()
	app, icon := tray.app, tray.icon
	tray.mu.Unlock()
	if app == nil || len(icon) == 0 {
		return
	}

	trayItem := app.SystemTray.New()
	trayItem.SetIcon(icon)
	trayItem.SetDarkModeIcon(icon)
	trayItem.SetTooltip("SniShaper")
	trayItem.OnClick(func() {
		a.RevealMainWindow()
	})

	menu := application.NewMenu()
	menu.Add("仪表盘").OnClick(func(ctx *application.Context) {
		a.RevealMainWindow()
	})
	menu.AddSeparator()

	proxyLabel := "代理: 关"
	if a.IsProxyRunning() {
		proxyLabel = "代理: 开"
	}
	proxyItem := menu.AddCheckbox(proxyLabel, a.IsProxyRunning())
	proxyItem.OnClick(func(ctx *application.Context) {
		a.RunSafeAsync("tray proxy toggle", func() {
			if a.IsProxyRunning() {
				_ = a.StopProxy()
			} else {
				_ = a.StartProxy()
			}
		})
	})

	systemProxyLabel := "系统代理: 关"
	if a.GetSystemProxyStatus().Enabled {
		systemProxyLabel = "系统代理: 开"
	}
	systemProxyItem := menu.Add(systemProxyLabel)
	systemProxyItem.OnClick(func(ctx *application.Context) {
		a.RunSafeAsync("tray system proxy toggle", func() {
			if a.GetSystemProxyStatus().Enabled {
				_ = a.DisableSystemProxy()
				return
			}
			if !a.IsProxyRunning() {
				if err := a.StartProxy(); err != nil {
					return
				}
			}
			_ = a.EnableSystemProxy()
		})
	})

	menu.AddSeparator()
	menu.Add("退出").OnClick(func(ctx *application.Context) {
		a.QuitApp()
	})
	trayItem.SetMenu(menu)

	a.systemTray = trayItem
	a.trayMenuV3 = menu
	a.proxyItemV3 = proxyItem
	a.systemProxyItemV3 = systemProxyItem

	tray.mu.Lock()
	tray.current = trayItem
	tray.lastReshow = time.Now()
	tray.mu.Unlock()
}

// RebuildSystemTray requests Wails to re-add the tray icon on the existing
// HWND/UID. Despite the name, it no longer creates a new tray: it triggers
// Wails' internal reshowSystrays() via a wmTaskbarCreated post, which is
// idempotent and never produces a duplicate icon. The name is kept so the
// existing call sites (wake, unlock, periodic refresh) stay unchanged.
func (a *App) RebuildSystemTray() {
	tray.mu.Lock()
	if tray.app == nil || time.Since(tray.lastReshow) < trayRebuildMinInterval {
		tray.mu.Unlock()
		return
	}
	tray.lastReshow = time.Now()
	tray.mu.Unlock()

	requestTrayReshow()
}

// trayRefreshLoop requests a reshow periodically to heal an icon that vanished
// while the window stayed open.
func (a *App) trayRefreshLoop() {
	for {
		time.Sleep(trayRebuildPeriod)
		if a.ShouldQuit() {
			return
		}
		a.RebuildSystemTray()
	}
}