package singtun

import (
	"fmt"
)

// singTunLogger 适配 sing-tun 的日志接口
type singTunLogger struct {
	logf func(string)
}

func (l *singTunLogger) Trace(args ...any) {
	if l.logf != nil {
		l.logf("[sing-tun:trace] " + fmt.Sprint(args...))
	}
}

func (l *singTunLogger) Debug(args ...any) {
	if l.logf != nil {
		l.logf("[sing-tun:debug] " + fmt.Sprint(args...))
	}
}

func (l *singTunLogger) Info(args ...any) {
	if l.logf != nil {
		l.logf("[sing-tun:info] " + fmt.Sprint(args...))
	}
}

func (l *singTunLogger) Warn(args ...any) {
	if l.logf != nil {
		l.logf("[sing-tun:warn] " + fmt.Sprint(args...))
	}
}

func (l *singTunLogger) Error(args ...any) {
	if l.logf != nil {
		l.logf("[sing-tun:error] " + fmt.Sprint(args...))
	}
}

func (l *singTunLogger) Fatal(args ...any) {
	if l.logf != nil {
		l.logf("[sing-tun:fatal] " + fmt.Sprint(args...))
	}
}

func (l *singTunLogger) Panic(args ...any) {
	if l.logf != nil {
		l.logf("[sing-tun:panic] " + fmt.Sprint(args...))
	}
}
