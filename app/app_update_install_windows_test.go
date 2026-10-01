//go:build windows

package app

import (
	"strings"
	"testing"
)

func TestPsQuoteSingle(t *testing.T) {
	if got := psQuoteSingle(`C:\Users\O'Brien\app`); got != `C:\Users\O''Brien\app` {
		t.Fatalf("unexpected escaping: %s", got)
	}
	if got := psQuoteSingle(`C:\plain\path`); got != `C:\plain\path` {
		t.Fatalf("plain path should stay unchanged, got: %s", got)
	}
}

func TestBuildUpdateScriptEscapesQuotes(t *testing.T) {
	script := buildUpdateScript(`C:\tmp\a'b.7z`, `C:\tmp\stage`, `C:\Users\x'y`, `C:\tmp\base`)
	// 未转义的单引号会截断 PowerShell 字符串，必须翻倍转义
	if strings.Contains(script, "a'b") || strings.Contains(script, "x'y") {
		t.Fatal("unescaped single quote in update script")
	}
	if !strings.Contains(script, "a''b") || !strings.Contains(script, "x''y") {
		t.Fatal("expected doubled single quotes in update script")
	}
}
