//go:build windows

package app

import (
	"strings"
	"testing"
)

func TestNormalizeAutoStartMethod(t *testing.T) {
	cases := map[string]string{
		"task":     "task",
		"service":  "service",
		"registry": "registry",
		"":         "task",
		"garbage":  "task",
		"TASK":     "task",
	}
	for in, want := range cases {
		if got := normalizeAutoStartMethod(in); got != want {
			t.Errorf("normalizeAutoStartMethod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBuildServiceBinaryPath(t *testing.T) {
	got := buildServiceBinaryPath(`C:\Program Files\SniShaper\SniShaper.exe`, []string{"--service-host", "--startup", "--autoproxy"})
	want := `"C:\Program Files\SniShaper\SniShaper.exe" --service-host --startup --autoproxy`
	if got != want {
		t.Fatalf("buildServiceBinaryPath() = %q, want %q", got, want)
	}
	// Every token must survive Windows' own unquoting.
	for _, token := range []string{`"C:\Program Files\SniShaper\SniShaper.exe"`, "--service-host", "--startup", "--autoproxy"} {
		if !strings.Contains(got, token) {
			t.Errorf("buildServiceBinaryPath() missing token %q", token)
		}
	}
}
