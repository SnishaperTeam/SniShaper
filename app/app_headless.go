//go:build headless

package app

// SetUIAdapter wires an optional event sink (window/tray in the GUI build).
func (a *App) SetUIAdapter(ui UIAdapter) { a.ui = ui }

// SetCLIMode marks the instance as headless; startup skips OS-level
// autostart registration that belongs to the desktop app.
func (a *App) SetCLIMode(enabled bool) { a.cliMode = enabled }

// SetSilentStdout suppresses stdout logging (used by the TUI so raw log
// lines never corrupt the terminal screen; logs still go to the ring
// buffer and log file).
func (a *App) SetSilentStdout(enabled bool) { a.silentStdout = enabled }

// invokeAsync runs fn in a fresh goroutine; the GUI build dispatches it
// onto the wails event loop instead.
func (a *App) invokeAsync(fn func()) {
	go fn()
}

// StartupCLI is the headless lifecycle entry point (the GUI build uses
// ServiceStartup). It runs the shared startupV3 sequence.
func (a *App) StartupCLI() error {
	a.startupV3()
	return nil
}

// ShutdownCLI performs the shared shutdown sequence in headless builds.
func (a *App) ShutdownCLI() {
	a.shutdown()
}

// Shared UI operations: headless no-ops. emitEvent forwards to the
// UIAdapter (if installed); the GUI build implements these on wails.

func (a *App) emitEvent(event string, payload interface{}) {
	if a.ui != nil {
		a.ui.Emit(event, payload)
	}
}

func (a *App) showMainWindow()                {}
func (a *App) hideMainWindow()                {}
func (a *App) minimiseMainWindow()            {}
func (a *App) toggleMaximiseMainWindow()      {}
func (a *App) closeMainWindow()               {}
func (a *App) HibernateMainWindow()           {}
func (a *App) quitAppUI()                     {}
func (a *App) setTrayTooltip(text string)     {}
func (a *App) updateTrayProxyItem(label string, checked bool) {}
func (a *App) updateTraySysProxyItem(label string)            {}

func (a *App) SetWailsApp(w any)              {}
func (a *App) SetMainWindow(w any)            {}
func (a *App) SetSystemTray(t any)            {}
func (a *App) SetTrayMenu(m any)              {}
func (a *App) SetProxyMenuItem(i any)         {}
func (a *App) SetSystemProxyMenuItem(i any)   {}
func (a *App) IsHibernated() bool             { return false }
func (a *App) IsMainWindowVisible() bool      { return false }
func (a *App) hasMainWindow() bool            { return false }

// StartInstanceListener and stopInstanceListener are no-ops without a window.
func (a *App) StartInstanceListener(string) {}

func (a *App) serveInstanceListener() {}

func stopInstanceListener() {}

// AttachMainWindowHandlers is a no-op without a window (see SetMainWindow).
func (a *App) AttachMainWindowHandlers(_ any) {}

// hasUI reports whether app state can be delivered to a UI. There is no window
// to track here, so state goes to the adapter a headless frontend installs.
func (a *App) hasUI() bool { return a.ui != nil }
