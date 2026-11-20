package cliapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseConfigValuePreservesExistingType 验证 config set 对已存在键保持
// 既有 JSON 类型（#198）：字符串键传数字仍写字符串，布尔/数字键严格解析。
func TestParseConfigValuePreservesExistingType(t *testing.T) {
	m := map[string]interface{}{
		"listen_port":    "8080", // string
		"auto_start":     true,   // bool
		"config_version": float64(2), // number
	}

	// 字符串键传数字 → 保持字符串。
	val, err := parseConfigValue("listen_port", "7890", m)
	if err != nil {
		t.Fatalf("parseConfigValue(listen_port): %v", err)
	}
	if s, ok := val.(string); !ok || s != "7890" {
		t.Fatalf("listen_port = %#v, want string \"7890\"", val)
	}

	// 布尔键传 true/false → 保持布尔。
	for _, raw := range []string{"true", "false"} {
		val, err = parseConfigValue("auto_start", raw, m)
		if err != nil {
			t.Fatalf("parseConfigValue(auto_start, %s): %v", raw, err)
		}
		if _, ok := val.(bool); !ok {
			t.Fatalf("auto_start = %#v, want bool", val)
		}
	}
	// 布尔键传非布尔 → 报错。
	if _, err := parseConfigValue("auto_start", "yes!", m); err == nil {
		t.Fatal("parseConfigValue(auto_start, yes!) should fail")
	}

	// 数字键传数字 → 保持数字。
	val, err = parseConfigValue("config_version", "3", m)
	if err != nil {
		t.Fatalf("parseConfigValue(config_version): %v", err)
	}
	if n, ok := val.(int); !ok || n != 3 {
		t.Fatalf("config_version = %#v, want int 3", val)
	}
	// 数字键传非数字 → 报错。
	if _, err := parseConfigValue("config_version", "abc", m); err == nil {
		t.Fatal("parseConfigValue(config_version, abc) should fail")
	}
}

// TestParseConfigValueNewKeyOrder 验证新键沿用 bool→int→float→string 顺序。
func TestParseConfigValueNewKeyOrder(t *testing.T) {
	m := map[string]interface{}{}

	val, _ := parseConfigValue("new_bool", "true", m)
	if b, ok := val.(bool); !ok || !b {
		t.Fatalf("new_bool = %#v, want true", val)
	}
	val, _ = parseConfigValue("new_int", "42", m)
	if n, ok := val.(int); !ok || n != 42 {
		t.Fatalf("new_int = %#v, want 42", val)
	}
	val, _ = parseConfigValue("new_float", "1.5", m)
	if f, ok := val.(float64); !ok || f != 1.5 {
		t.Fatalf("new_float = %#v, want 1.5", val)
	}
	val, _ = parseConfigValue("new_str", "hello", m)
	if s, ok := val.(string); !ok || s != "hello" {
		t.Fatalf("new_str = %#v, want \"hello\"", val)
	}
}

// TestValidateConfigValueListenPort 验证 listen_port 越界（0/70000）被拒绝。
func TestValidateConfigValueListenPort(t *testing.T) {
	for _, bad := range []string{"0", "70000", "abc"} {
		if err := validateConfigValue("listen_port", bad); err == nil {
			t.Fatalf("validateConfigValue(listen_port, %q) should fail", bad)
		}
	}
	for _, good := range []string{"1", "8080", "65535"} {
		if err := validateConfigValue("listen_port", good); err != nil {
			t.Fatalf("validateConfigValue(listen_port, %q): %v", good, err)
		}
	}
}

// TestValidateConfigValueListenAddress 验证 listen_address 仅接受空或合法 IP。
func TestValidateConfigValueListenAddress(t *testing.T) {
	for _, bad := range []string{"abc", "999.1.1.1", "192.168.1"} {
		if err := validateConfigValue("listen_address", bad); err == nil {
			t.Fatalf("validateConfigValue(listen_address, %q) should fail", bad)
		}
	}
	for _, good := range []string{"", "127.0.0.1", "0.0.0.0", "192.168.1.10", "::1", "fe80::1"} {
		if err := validateConfigValue("listen_address", good); err != nil {
			t.Fatalf("validateConfigValue(listen_address, %q): %v", good, err)
		}
	}
}

// TestOpConfigSetUnknownKeyWarnsButWrites 验证未知键警告不阻断写入。
func TestOpConfigSetUnknownKeyWarnsButWrites(t *testing.T) {
	// settingsPaths() 解析到进程 CWD；切到临时目录，避免污染包目录。
	t.Chdir(t.TempDir())
	settingsPath, _ := settingsPaths()
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	seed := `{"listen_port":"8080"}`
	if err := os.WriteFile(settingsPath, []byte(seed), 0644); err != nil {
		t.Fatal(err)
	}

	var lines []string
	code := opConfig([]string{"set", "some_future_key", "hello"}, func(s string) { lines = append(lines, s) })
	if code != 0 {
		t.Fatalf("opConfig set unknown key exit = %d, want 0; lines: %v", code, lines)
	}

	warned := false
	for _, l := range lines {
		if strings.Contains(l, "警告") {
			warned = true
		}
	}
	if !warned {
		t.Fatalf("expected unknown-key warning, lines: %v", lines)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["some_future_key"] != "hello" {
		t.Fatalf("unknown key not written, file: %s", data)
	}
	if m["listen_port"] != "8080" {
		t.Fatalf("listen_port corrupted, file: %s", data)
	}
}
