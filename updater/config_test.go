//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInstalledMissingKey(t *testing.T) {
	info := readInstalled(Config{AppID: "{00000000-0000-0000-0000-000000000000}"})
	if info.Version != "" || info.Dir != "" {
		t.Fatalf("unexpected installed info: %+v", info)
	}
}

func TestLoadConfigMinimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "updater.json")
	// 旧版 updater.json 里的 manifest/公钥等字段现在会被静默忽略。
	if err := os.WriteFile(path, []byte(`{"app_id":"{00000000-0000-0000-0000-000000000000}","manifest_url":"https://example.com/m.json","public_key":"xx"}`), 0644); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(path)
	if err != nil {
		t.Fatalf("minimal config rejected: %v", err)
	}
	if c.AppID != "{00000000-0000-0000-0000-000000000000}" {
		t.Fatalf("app_id = %q", c.AppID)
	}
	if c.MaxExtractBytes <= 0 {
		t.Fatalf("max_extract_bytes default missing: %d", c.MaxExtractBytes)
	}

	if err := os.WriteFile(path, []byte(`{"max_extract_bytes":100}`), 0644); err != nil {
		t.Fatal(err)
	}
	c, err = loadConfig(path)
	if err != nil {
		t.Fatalf("default app_id must be accepted: %v", err)
	}
	if c.AppID != "{3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D4}" {
		t.Fatalf("app_id default missing: %q", c.AppID)
	}
}
