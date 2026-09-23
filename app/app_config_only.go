package app

import (
	"log"
	"os"
	"path/filepath"

	"snishaper/common"
)

// StartupConfigOnly prepares logging and loads the settings and rules without
// starting the core process, the UI or any monitor. It deliberately opens no
// log file and keeps loader output off stdout, so a one-shot CLI command stays
// quiet and does not litter the log directory.
func (a *App) StartupConfigOnly() error {
	if a.logBuffer == nil {
		a.logBuffer = common.NewRingLogWriter(500)
	}
	log.SetOutput(&gatedLogWriter{app: a})
	a.resolveLogDir()
	return a.ruleManager.LoadConfig()
}

// resolveLogDir points the app at <execDir>/log without creating a log file, so
// commands that read or clean existing logs still find them.
func (a *App) resolveLogDir() {
	if a.logDir != "" {
		return
	}
	ep, err := os.Executable()
	if err != nil {
		return
	}
	a.logDir = filepath.Join(filepath.Dir(ep), "log")
}
