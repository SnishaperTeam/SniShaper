package singtun

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// logfHandler 把 slog 记录桥接回 App 层注入的 logf func(string) 管道。
//
// slog 仅限 pkg/singtun 内部使用（设计决策 D9）：对外 NewManager(resolver, logf)
// 签名不变，所有日志最终仍经 logf 进入 App 日志管道，App 层零改动。
// 分级体现在消息前缀（[sing-tun] / [sing-tun:debug] / [sing-tun:warn] /
// [sing-tun:error]），logf 管道本身不过滤，保持切换前"全部输出"的行为。
type logfHandler struct {
	logf func(string)
}

// newBridgedLogger 构建输出到 logf 的 *slog.Logger。logf 为 nil 时所有日志静默丢弃。
func newBridgedLogger(logf func(string)) *slog.Logger {
	return slog.New(&logfHandler{logf: logf})
}

func (h *logfHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h.logf != nil
}

func (h *logfHandler) Handle(_ context.Context, r slog.Record) error {
	if h.logf == nil {
		return nil
	}
	var b strings.Builder
	switch {
	case r.Level >= slog.LevelError:
		b.WriteString("[sing-tun:error] ")
	case r.Level >= slog.LevelWarn:
		b.WriteString("[sing-tun:warn] ")
	case r.Level >= slog.LevelInfo:
		b.WriteString("[sing-tun] ")
	default:
		b.WriteString("[sing-tun:debug] ")
	}
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		b.WriteByte(' ')
		b.WriteString(a.Key)
		b.WriteByte('=')
		b.WriteString(a.Value.String())
		return true
	})
	h.logf(b.String())
	return nil
}

func (h *logfHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *logfHandler) WithGroup(string) slog.Handler      { return h }

// singTunLogger 适配 sing-tun 的日志接口，输出经 slog 分级后桥接回 logf。
type singTunLogger struct {
	logger *slog.Logger
}

func (l *singTunLogger) Trace(args ...any) { l.output(slog.LevelDebug, "trace", args...) }
func (l *singTunLogger) Debug(args ...any) { l.output(slog.LevelDebug, "debug", args...) }
func (l *singTunLogger) Info(args ...any)  { l.output(slog.LevelInfo, "info", args...) }
func (l *singTunLogger) Warn(args ...any)  { l.output(slog.LevelWarn, "warn", args...) }
func (l *singTunLogger) Error(args ...any) { l.output(slog.LevelError, "error", args...) }
func (l *singTunLogger) Fatal(args ...any) { l.output(slog.LevelError, "fatal", args...) }
func (l *singTunLogger) Panic(args ...any) { l.output(slog.LevelError, "panic", args...) }

func (l *singTunLogger) output(level slog.Level, tag string, args ...any) {
	if l.logger == nil {
		return
	}
	msg := tag
	if len(args) > 0 {
		msg = tag + ": " + fmt.Sprint(args...)
	}
	l.logger.Log(context.Background(), level, msg)
}
