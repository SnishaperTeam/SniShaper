package app

import (
	"context"
	"log/slog"
	"strings"
)

// wailsLogHandler routes the runtime's system messages into the app log. The
// desktop build has no console, so the runtime's own logger (stderr) is
// otherwise invisible - that is where notification icon and window failures
// are reported.
type wailsLogHandler struct{ app *App }

// Enabled keeps warnings and errors and drops the runtime's info chatter, which
// is dominated by one line per served asset.
func (h wailsLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h wailsLogHandler) Handle(_ context.Context, record slog.Record) error {
	var b strings.Builder
	b.WriteString("[wails] ")
	b.WriteString(strings.ToLower(record.Level.String()))
	b.WriteString(": ")
	b.WriteString(record.Message)
	record.Attrs(func(attr slog.Attr) bool {
		b.WriteString(" ")
		b.WriteString(attr.Key)
		b.WriteString("=")
		b.WriteString(attr.Value.String())
		return true
	})
	h.app.appendLog(b.String())
	return nil
}

func (h wailsLogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h wailsLogHandler) WithGroup(_ string) slog.Handler      { return h }

// FrameworkLogger is the logger handed to the wails runtime.
func (a *App) FrameworkLogger() *slog.Logger { return slog.New(wailsLogHandler{app: a}) }
