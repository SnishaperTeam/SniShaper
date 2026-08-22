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
func (a *App) quitAppUI()                     {}
func (a *App) setTrayTooltip(text string)     {}
func (a *App) updateTrayProxyItem(label string, checked bool) {}
func (a *App) updateTraySysProxyItem(label string)            {}
