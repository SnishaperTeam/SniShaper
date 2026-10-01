package certmanager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCACorruptedCertPEMReturnsError(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	if err := os.WriteFile(caPath, []byte("not a pem file"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("not a pem file"), 0644); err != nil {
		t.Fatal(err)
	}
	cm := NewCertManager(caPath, keyPath)
	if err := cm.LoadCA(); err == nil {
		t.Fatal("expected error for corrupted CA pem, got nil")
	}
}

func TestLoadCACorruptedKeyPEMReturnsError(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	keyPath := filepath.Join(dir, "ca.key")
	cm := NewCertManager(caPath, keyPath)
	if err := cm.GenerateCA(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("garbage"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := cm.LoadCA(); err == nil {
		t.Fatal("expected error for corrupted key pem, got nil")
	}
}
