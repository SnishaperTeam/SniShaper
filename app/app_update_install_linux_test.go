//go:build linux

package app

import (
	"strings"
	"testing"
)

func TestShQuote(t *testing.T) {
	if got := shQuote(`/opt/SniShaper's dir`); got != `'/opt/SniShaper'\''s dir'` {
		t.Fatalf("unexpected escaping: %s", got)
	}
	if got := shQuote("/opt/plain"); got != "'/opt/plain'" {
		t.Fatalf("plain path should stay quoted, got: %s", got)
	}
}

func TestBuildUpdateApplyScriptQuotesPaths(t *testing.T) {
	script := buildUpdateApplyScript(4242, "/tmp/s'stage", "/opt/a'pp", "/opt/a'pp/snishaper", "/tmp/s'base")
	// 未转义的单引号会截断 sh 字符串，必须转义
	if strings.Contains(script, "s'stage") || strings.Contains(script, "a'pp") || strings.Contains(script, "s'base") {
		t.Fatal("unescaped single quote in apply script")
	}
	if !strings.Contains(script, "4242") {
		t.Fatal("pid missing from apply script")
	}
	for _, want := range []string{"kill -0 4242", "cp -a", "--startup", "rm -rf"} {
		if !strings.Contains(script, want) {
			t.Fatalf("apply script missing %q", want)
		}
	}
}
