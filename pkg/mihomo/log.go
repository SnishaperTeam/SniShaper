package mihomo

import (
	"strings"
	"sync"
)

type ringLogWriter struct {
	mu      sync.RWMutex
	lines   []string
	pending string
	max     int
}

func newRingLogWriter(max int) *ringLogWriter {
	if max <= 0 {
		max = 1000
	}
	return &ringLogWriter{
		lines: make([]string, 0, max),
		max:   max,
	}
}

func (w *ringLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	text := w.pending + strings.ReplaceAll(string(p), "\r\n", "\n")
	parts := strings.Split(text, "\n")
	if len(parts) == 0 {
		return len(p), nil
	}
	w.pending = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		if line == "" {
			continue
		}
		w.lines = append(w.lines, line)
		if len(w.lines) > w.max {
			if cap(w.lines) > w.max*2 {
				newLines := make([]string, w.max)
				copy(newLines, w.lines[len(w.lines)-w.max:])
				w.lines = newLines
			} else {
				w.lines = w.lines[len(w.lines)-w.max:]
			}
		}
	}
	return len(p), nil
}

func (w *ringLogWriter) Snapshot(limit int) []string {
	if limit <= 0 {
		limit = 200
	}
	w.mu.RLock()
	defer w.mu.RUnlock()

	total := len(w.lines)
	if total == 0 {
		if w.pending != "" {
			return []string{w.pending}
		}
		return []string{}
	}
	if limit > total {
		limit = total
	}
	start := total - limit
	out := make([]string, limit)
	copy(out, w.lines[start:])

	if w.pending != "" {
		out = append(out, w.pending)
		if len(out) > limit {
			out = out[1:]
		}
	}
	return out
}

func (w *ringLogWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lines = w.lines[:0]
	w.pending = ""
}
