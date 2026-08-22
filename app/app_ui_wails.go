//go:build !headless

package app

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Typed accessors for the GUI-only UI references (stored as `any` on App
// so the headless build never links the wails runtime).

func (a *App) wailsAppInstance() *application.App {
	if a.wailsApp == nil {
		return nil
	}
	return a.wailsApp.(*application.App)
}

func (a *App) mainWindowInstance() *application.WebviewWindow {
	if a.mainWindow == nil {
		return nil
	}
	return a.mainWindow.(*application.WebviewWindow)
}

func (a *App) systemTrayInstance() *application.SystemTray {
	if a.systemTray == nil {
		return nil
	}
	return a.systemTray.(*application.SystemTray)
}

func (a *App) trayMenuV3Instance() *application.Menu {
	if a.trayMenuV3 == nil {
		return nil
	}
	return a.trayMenuV3.(*application.Menu)
}

func (a *App) proxyItemV3Instance() *application.MenuItem {
	if a.proxyItemV3 == nil {
		return nil
	}
	return a.proxyItemV3.(*application.MenuItem)
}

func (a *App) systemProxyItemV3Instance() *application.MenuItem {
	if a.systemProxyItemV3 == nil {
		return nil
	}
	return a.systemProxyItemV3.(*application.MenuItem)
}

// invokeAsync runs fn on the wails event loop; the headless build
// executes it directly in its own goroutine.
func (a *App) invokeAsync(fn func()) {
	application.InvokeAsync(fn)
}

// SetWailsApp sets the wails application instance.
func (a *App) SetWailsApp(w *application.App) { a.wailsApp = w }

// SetMainWindow sets the main window reference.
func (a *App) SetMainWindow(w *application.WebviewWindow) {
	if w != nil {
		a.mainWindow = w
		if a.pendingShow {
			a.mainWindowInstance().Show()
			a.mainWindowInstance().Focus()
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

// wails service lifecycle entry points.

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.startupV3()
	return nil
}

func (a *App) ServiceShutdown() error {
	a.shutdown()
	return nil
}

// Shared UI operations: the GUI build dispatches onto wails; the headless
// build (app_headless.go) provides no-op equivalents, forwarding events to
// the UIAdapter when one is installed.

func (a *App) emitEvent(event string, payload interface{}) {
	if a.mainWindow != nil {
		a.mainWindowInstance().EmitEvent(event, payload)
	}
}

func (a *App) showMainWindow() {
	if a.mainWindow != nil {
		a.mainWindowInstance().Show()
		a.mainWindowInstance().Focus()
	}
}

func (a *App) hideMainWindow() {
	if a.mainWindow != nil {
		a.mainWindowInstance().Hide()
	}
}

func (a *App) minimiseMainWindow() {
	if a.mainWindow != nil {
		a.mainWindowInstance().Minimise()
	}
}

func (a *App) toggleMaximiseMainWindow() {
	if a.mainWindow != nil {
		a.mainWindowInstance().ToggleMaximise()
	}
}

func (a *App) closeMainWindow() {
	if a.mainWindow != nil {
		a.mainWindowInstance().Close()
	}
}

func (a *App) quitAppUI() {
	if a.wailsApp != nil {
		a.wailsAppInstance().Quit()
	}
}

func (a *App) setTrayTooltip(text string) {
	if a.systemTray != nil {
		a.systemTrayInstance().SetTooltip(text)
	}
}

func (a *App) updateTrayProxyItem(label string, checked bool) {
	if a.proxyItemV3 != nil {
		a.proxyItemV3Instance().SetLabel(label)
		a.proxyItemV3Instance().SetChecked(checked)
	}
}

func (a *App) updateTraySysProxyItem(label string) {
	if a.systemProxyItemV3 != nil {
		a.systemProxyItemV3Instance().SetLabel(label)
	}
}
