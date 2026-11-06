//go:build headless

package app

// headlessBuild marks the CLI flavour, whose update list only contains the
// snishaper-cli-* archives instead of the desktop packages.
const headlessBuild = true

// invokeAsync runs fn in a fresh goroutine; the GUI build dispatches it
// onto the wails event loop instead.
func (a *App) invokeAsync(fn func()) {
	go fn()
}

// Shared UI operations: headless no-ops. emitEvent forwards to the
// UIAdapter (if installed); the GUI build implements these on wails.

func (a *App) emitEvent(event string, payload interface{}) {
	if a.ui != nil {
		a.ui.Emit(event, payload)
	}
}

func (a *App) showMainWindow()                                {}
func (a *App) hideMainWindow()                                {}
func (a *App) minimiseMainWindow()                            {}
func (a *App) toggleMaximiseMainWindow()                      {}
func (a *App) closeMainWindow()                               {}
func (a *App) HibernateMainWindow()                           {}
func (a *App) quitAppUI()                                     {}
func (a *App) setTrayTooltip(text string)                     {}
func (a *App) updateTrayProxyItem(label string, checked bool) {}
func (a *App) updateTraySysProxyItem(label string)            {}

func (a *App) SetWailsApp(w any)            {}
func (a *App) SetMainWindow(w any)          {}
func (a *App) SetSystemTray(t any)          {}
func (a *App) SetTrayMenu(m any)            {}
func (a *App) SetProxyMenuItem(i any)       {}
func (a *App) SetSystemProxyMenuItem(i any) {}
func (a *App) IsHibernated() bool           { return false }
func (a *App) IsMainWindowVisible() bool    { return false }
func (a *App) hasMainWindow() bool          { return false }

// StartInstanceListener and stopInstanceListener are no-ops without a window.
func (a *App) StartInstanceListener(string) {}

func (a *App) serveInstanceListener() {}

func stopInstanceListener() {}

// AttachMainWindowHandlers is a no-op without a window (see SetMainWindow).
func (a *App) AttachMainWindowHandlers(_ any) {}

// hasUI reports whether app state can be delivered to a UI. There is no window
// to track here, so state goes to the adapter a headless frontend installs.
func (a *App) hasUI() bool { return a.ui != nil }
