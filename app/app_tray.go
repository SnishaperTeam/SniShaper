//go:build !headless

package app

import (
	"log"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// The runtime adds the tray icon once and never re-adds it: its own icon update
// reports a failed Shell_NotifyIcon and rolls back instead of registering the
// icon again. The shell drops icons for reasons the app cannot observe (a
// display or session change, a shell glitch, or a notification area that simply
// forgets the icon), so the app owns the tray lifecycle: it builds the tray at
// startup and rebuilds it whenever the window goes away or a long interval
// passes.

const (
	trayRebuildMinInterval = 3 * time.Second
	trayRebuildPeriod      = 30 * time.Minute
)

type trayHolder struct {
	mu         sync.Mutex
	app        *application.App
	icon       []byte
	current    *application.SystemTray
	builtAt    time.Time
	rebuilding bool
}

var tray trayHolder

// BuildSystemTray creates the tray icon and starts the periodic refresh. It must
// be called before the runtime starts, so the tray is registered while the app
// is still starting up.
func (a *App) BuildSystemTray(wailsApp *application.App, icon []byte) {
	tray.mu.Lock()
	tray.app = wailsApp
	tray.icon = icon
	tray.mu.Unlock()

	a.buildSystemTray()
	go a.trayRefreshLoop()
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
	tray.builtAt = time.Now()
	tray.mu.Unlock()
}

// RebuildSystemTray replaces the tray icon with a fresh one, which is the only
// way to recover an icon the shell dropped. It returns immediately and does the
// work on the runtime's main thread.
func (a *App) RebuildSystemTray() {
	tray.mu.Lock()
	if tray.rebuilding || tray.app == nil || time.Since(tray.builtAt) < trayRebuildMinInterval {
		tray.mu.Unlock()
		return
	}
	tray.rebuilding = true
	old := tray.current
	tray.current = nil
	tray.mu.Unlock()

	go func() {
		defer func() {
			tray.mu.Lock()
			tray.rebuilding = false
			tray.mu.Unlock()
		}()

		// Build the replacement first and drop the old icon afterwards: a window
		// with no tray icon cannot be brought back, so the gap must not exist.
		application.InvokeSync(func() {
			a.buildSystemTray()
			if old != nil {
				old.Destroy()
			}
		})

		tray.mu.Lock()
		rebuilt := tray.current != nil && tray.current != old
		tray.mu.Unlock()
		if rebuilt {
			log.Printf("[tray] system tray rebuilt")
			return
		}

		// Registration failed: retry shortly instead of waiting a full period.
		log.Printf("[tray] system tray rebuild did not take effect, retrying")
		tray.mu.Lock()
		tray.builtAt = time.Time{}
		tray.mu.Unlock()
		time.Sleep(30 * time.Second)
		a.RebuildSystemTray()
	}()
}

// trayRefreshLoop heals an icon that vanished while the window stayed open.
func (a *App) trayRefreshLoop() {
	for {
		time.Sleep(trayRebuildPeriod)
		if a.ShouldQuit() {
			return
		}
		a.RebuildSystemTray()
	}
}
