package app

// CLI-mode lifecycle shared by the headless CLI binary and the desktop
// binary's CLI compatibility mode. These used to live behind the `headless`
// build tag; the desktop binary now also runs the CLI dispatcher (see
// main.go), so both flavours need the same setters and entry points.

// SetUIAdapter wires an optional event sink (window/tray in the GUI build).
func (a *App) SetUIAdapter(ui UIAdapter) { a.ui = ui }

// SetCLIMode marks the instance as headless; startup skips OS-level
// autostart registration that belongs to the desktop app.
func (a *App) SetCLIMode(enabled bool) { a.cliMode = enabled }

// SetSilentStdout suppresses stdout logging (used by the TUI so raw log
// lines never corrupt the terminal screen; logs still go to the ring
// buffer and log file).
func (a *App) SetSilentStdout(enabled bool) { a.silentStdout = enabled }

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
