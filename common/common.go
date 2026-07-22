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
