package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"snishaper/common"
)

// enableCrashLog points the Go runtime crash output at a file next to the run
// log, so an unexpected death leaves a stack trace behind instead of vanishing.
// It returns the path that was opened, or an empty string when it failed.
func (a *App) enableCrashLog() string {
	if path := common.CrashLogPath(); path != "" {
		return path
	}
	dir := a.logDir
	if dir == "" {
		a.resolveLogDir()
		dir = a.logDir
	}
	return common.EnableCrashLog(dir)
}

// CrashLogPath reports the file that receives a crash report, if enabled.
func CrashLogPath() string {
	return common.CrashLogPath()
}

// noteCrashLog records where a crash report would land, so a user reading the
// log knows where to look after the app disappears.
func (a *App) noteCrashLog() {
	path := a.enableCrashLog()
	if path == "" {
		return
	}
	fmt.Println("[startup] Crash report file: " + path)
	a.appendLog("[startup] Crash report file: " + path)
}

// reportPreviousRun detects a previous process that never reached its shutdown
// hook: such a run leaves its marker behind. A marker that is still present at
// startup means the last instance crashed or was terminated from the outside,
// which is the difference between a bug report and an antivirus kill.
func (a *App) reportPreviousRun() {
	previous := readRunMarker(a.logDir)
	writeRunMarker(a.logDir)
	if previous == "" {
		return
	}
	a.appendLog("[startup] Previous run did not shut down cleanly (" + previous + ")")
}

// clearRunMarker is called from the shutdown hook, so a clean exit leaves no
// trace and the next startup stays quiet.
func (a *App) clearRunMarker() {
	if a.logDir == "" {
		return
	}
	_ = os.Remove(runMarkerPath(a.logDir))
}

func runMarkerPath(dir string) string {
	return filepath.Join(dir, "run.marker")
}

func writeRunMarker(dir string) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	content := fmt.Sprintf("pid=%d started=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
	_ = os.WriteFile(runMarkerPath(dir), []byte(content), 0644)
}

// readRunMarker returns a report line for a stale marker, or an empty string
// when there is nothing to report.
func readRunMarker(dir string) string {
	if dir == "" {
		return ""
	}
	data, err := os.ReadFile(runMarkerPath(dir))
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	text = strings.ReplaceAll(text, "\n", ", ")
	text = strings.ReplaceAll(text, "pid=", "pid ")
	text = strings.ReplaceAll(text, "started=", "started ")
	return text
}
