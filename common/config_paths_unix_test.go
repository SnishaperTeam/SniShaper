//go:build linux || darwin

package common

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupPortableEnv 准备隔离的用户环境（XDG/HOME/SUDO_USER）并返回
// execDir 与 XDG 数据根目录（<home>/.config/snishaper）。
func setupPortableEnv(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("HOME", home)
	t.Setenv("SUDO_USER", "")
	execDir := t.TempDir()
	return execDir, filepath.Join(home, ".config", "snishaper")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPortablePathsFreshExtractUsesExecDir(t *testing.T) {
	execDir, xdgRoot := setupPortableEnv(t)

	// 发行包种子文件在二进制旁（无任何 XDG 旧数据）。
	writeFile(t, filepath.Join(execDir, "config", "settings.json"), `{"listen_port":"8080"}`)
	writeFile(t, filepath.Join(execDir, "rules", "config.json"), `{"seed":true}`)

	if got := ConfigSettingsPath(execDir); got != filepath.Join(execDir, "config", "settings.json") {
		t.Fatalf("ConfigSettingsPath = %q, want execDir path", got)
	}
	if got := ConfigRulesPath(execDir); got != filepath.Join(execDir, "rules", "config.json") {
		t.Fatalf("ConfigRulesPath = %q, want execDir path", got)
	}
	if got := ConfigProxyMarker(execDir); got != filepath.Join(execDir, "config", "system_proxy_owner.json") {
		t.Fatalf("ConfigProxyMarker = %q, want execDir path", got)
	}
	if _, err := os.Stat(xdgRoot); !os.IsNotExist(err) {
		t.Fatalf("no XDG data should be created for a fresh extract: %v", err)
	}
}

func TestPortableMigrationOverwritesSeedsAndCleansXDG(t *testing.T) {
	execDir, xdgRoot := setupPortableEnv(t)

	// 发行包种子文件（应被 XDG 用户数据覆盖）。
	writeFile(t, filepath.Join(execDir, "config", "settings.json"), `{"listen_port":"8080"}`)
	// XDG 用户数据（权威源）。
	writeFile(t, filepath.Join(xdgRoot, "config", "settings.json"), `{"listen_port":"12345"}`)
	writeFile(t, filepath.Join(xdgRoot, "config", "system_proxy_owner.json"), `{"pid":1}`)
	writeFile(t, filepath.Join(xdgRoot, "rules", "config.json"), `{"user":"data"}`)
	// 证书不迁移，必须保留在 XDG 侧。
	writeFile(t, filepath.Join(xdgRoot, "cert", "ca.crt"), "CERT")

	if got := ConfigSettingsPath(execDir); got != filepath.Join(execDir, "config", "settings.json") {
		t.Fatalf("ConfigSettingsPath = %q, want execDir path after migration", got)
	}
	if data := mustRead(t, filepath.Join(execDir, "config", "settings.json")); !strings.Contains(data, "12345") {
		t.Fatalf("XDG settings should win over seed, got %s", data)
	}
	if data := mustRead(t, filepath.Join(execDir, "config", "system_proxy_owner.json")); !strings.Contains(data, "pid") {
		t.Fatalf("proxy marker should migrate with config dir, got %s", data)
	}
	if data := mustRead(t, ConfigRulesPath(execDir)); !strings.Contains(data, `"user":"data"`) {
		t.Fatalf("rules should migrate to execDir, got %s", data)
	}
	if _, err := os.Stat(filepath.Join(xdgRoot, "config")); !os.IsNotExist(err) {
		t.Fatalf("XDG config dir should be removed after migration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(xdgRoot, "rules")); !os.IsNotExist(err) {
		t.Fatalf("XDG rules dir should be removed after migration: %v", err)
	}
	if _, err := os.Stat(filepath.Join(xdgRoot, "cert", "ca.crt")); err != nil {
		t.Fatalf("cert must stay untouched in XDG dir: %v", err)
	}
}

func TestPortableMigrationIdempotent(t *testing.T) {
	execDir, xdgRoot := setupPortableEnv(t)

	writeFile(t, filepath.Join(xdgRoot, "config", "settings.json"), `{"listen_port":"12345"}`)
	first := ConfigSettingsPath(execDir)
	second := ConfigSettingsPath(execDir)
	if first != second {
		t.Fatalf("repeated resolution moved: %q vs %q", first, second)
	}
	if data := mustRead(t, second); !strings.Contains(data, "12345") {
		t.Fatalf("settings content changed on second call: %s", data)
	}
	if _, err := os.Stat(filepath.Join(xdgRoot, "config")); !os.IsNotExist(err) {
		t.Fatalf("XDG config dir should stay removed: %v", err)
	}
}

func TestPortableFallbackWhenExecDirUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions; cannot simulate unwritable execDir")
	}
	_, xdgRoot := setupPortableEnv(t)

	// 只读 execDir。
	base := t.TempDir()
	execDir := filepath.Join(base, "exec")
	if err := os.MkdirAll(execDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(execDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(execDir, 0755) }()

	writeFile(t, filepath.Join(xdgRoot, "config", "settings.json"), `{"listen_port":"12345"}`)

	got := ConfigSettingsPath(execDir)
	if got != filepath.Join(xdgRoot, "config", "settings.json") {
		t.Fatalf("ConfigSettingsPath = %q, want XDG fallback path", got)
	}
	if data := mustRead(t, got); !strings.Contains(data, "12345") {
		t.Fatalf("fallback should read XDG data, got %s", data)
	}
	if _, err := os.Stat(filepath.Join(xdgRoot, "config", "settings.json")); err != nil {
		t.Fatalf("XDG data must be preserved on fallback: %v", err)
	}
}
