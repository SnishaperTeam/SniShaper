//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

func NewLogger(dir string) *Logger {
	dirs := []string{}
	if dir != "" {
		dirs = append(dirs, dir)
	}
	dirs = append(dirs, filepath.Join(os.TempDir(), "snishaper-updater"))
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			continue
		}
		p := filepath.Join(d, "updater.log")
		f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		return &Logger{path: p, f: f}
	}
	return &Logger{}
}

func (l *Logger) Path() string {
	return l.path
}

func (l *Logger) Logf(format string, args ...any) {
	line := fmt.Sprintf("%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), fmt.Sprintf(format, args...))
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		_, _ = l.f.WriteString(line)
		_ = l.f.Sync()
	}
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
}
