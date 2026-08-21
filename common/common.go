package common

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type RingLogWriter struct {
	mu       sync.Mutex
	lines    []string
	capacity int
	size     int
	start    int
}

func NewRingLogWriter(capacity int) *RingLogWriter {
	return &RingLogWriter{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

func (w *RingLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	msg := string(p)
	lines := strings.Split(msg, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if w.size < w.capacity {
			w.lines[w.size] = trimmed
			w.size++
		} else {
			w.lines[w.start] = trimmed
			w.start = (w.start + 1) % w.capacity
		}
	}
	return len(p), nil
}

func (w *RingLogWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.start = 0
	w.size = 0
}

func (w *RingLogWriter) Snapshot(limit int) []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	if limit <= 0 || limit > w.size {
		limit = w.size
	}

	result := make([]string, limit)
	if w.size < w.capacity {
		copy(result, w.lines[w.size-limit:w.size])
	} else {
		for i := 0; i < limit; i++ {
			idx := (w.start + w.size - limit + i) % w.capacity
			result[i] = w.lines[idx]
		}
	}
	return result
}

func ResolveRuntimeFile(execDir, relPath string) string {
	absPath := filepath.Join(execDir, relPath)
	if _, err := os.Stat(absPath); err == nil {
		return absPath
	}
	fallback := filepath.Join(".", relPath)
	return fallback
}

// UserConfigDir returns the SniShaper-specific user config directory
// (e.g. ~/.config/snishaper on Linux / XDG). Falls back to $HOME/.config
// when os.UserConfigDir fails.
//
// When the app runs elevated (root via sudo), os.UserConfigDir resolves to
// /root/.config, which would split user data away from the desktop session.
// In that case the directory of $SUDO_USER is used instead so settings,
// rules and certificates stay in the invoking user's home.
func UserConfigDir() string {
	dir := ""
	if user := os.Getenv("SUDO_USER"); user != "" && user != "root" {
		home := "/home/" + user
		if h, err := os.UserHomeDir(); err == nil && strings.HasSuffix(h, user) {
			home = h
		}
		if _, err := os.Stat(home); err == nil {
			dir = filepath.Join(home, ".config")
		}
	}
	if dir == "" {
		if d, err := os.UserConfigDir(); err == nil && d != "" {
			dir = d
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			dir = filepath.Join(home, ".config")
		}
	}
	if dir == "" {
		dir = "./.config"
	}
	return filepath.Join(dir, "snishaper")
}

// UserConfigPath maps a runtime-relative path (e.g. config/settings.json)
// into the per-user config directory.
func UserConfigPath(relPath string) string {
	return filepath.Join(UserConfigDir(), relPath)
}

// EnsureUserConfig resolves a config file into the per-user config directory.
// On first run the bundled default shipped next to the executable is copied
// into the user directory so the user edits survive updates.
func EnsureUserConfig(execDir, relPath string) (string, error) {
	userPath := UserConfigPath(relPath)
	if _, err := os.Stat(userPath); err == nil {
		return userPath, nil
	}
	if err := os.MkdirAll(filepath.Dir(userPath), 0755); err != nil {
		return userPath, err
	}
	if execDir != "" {
		if data, err := os.ReadFile(filepath.Join(execDir, relPath)); err == nil {
			if err := os.WriteFile(userPath, data, 0644); err != nil {
				return userPath, err
			}
		}
	}
	return userPath, nil
}
