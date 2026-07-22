package clashprovider_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/metacubex/mihomo/adapter/provider"
	"github.com/metacubex/mihomo/component/resolver"
)

func TestNewHTTPProviderEmptyURL(t *testing.T) {
	defer resolver.Pause()
	p, err := newHTTPProvider(t.TempDir(), providerConfig{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "test" {
		t.Fatal("expected name test")
	}
}

func TestNewHTTPProviderReadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "clash_provider.yaml")
	if err := os.WriteFile(filePath, []byte("payload:\n- http://proxy1.com\n- http://proxy2.com\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := newHTTPProvider(dir, providerConfig{Name: "test", File: filePath})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "test" {
		t.Fatal("expected name test")
	}
}
