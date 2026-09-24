package common

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// crashLogFile stays open for the lifetime of the process: the Go runtime
// writes its fatal error report (a panic in any goroutine, a deadlock, a fatal
// signal) into it. That report would otherwise go to stderr, which the desktop
// build and the core child process have nowhere to send.
var crashLogFile *os.File

// EnableCrashLog opens a crash report file in dir and points the runtime at it.
// It returns the path, or an empty string when the file could not be opened.
func EnableCrashLog(dir string) string {
	if crashLogFile != nil {
		return crashLogFile.Name()
	}
	if dir == "" {
		return ""
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ""
	}

	name := "crash-" + time.Now().Format("2006-01-02_15-04-05") + ".log"
	file, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return ""
	}
	if err := debug.SetCrashOutput(file, debug.CrashOptions{}); err != nil {
		_ = file.Close()
		return ""
	}

	crashLogFile = file
	return file.Name()
}

// CrashLogPath reports the file that receives a crash report, if enabled.
func CrashLogPath() string {
	if crashLogFile == nil {
		return ""
	}
	return crashLogFile.Name()
}
