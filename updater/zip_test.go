//go:build windows

package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeTestZip(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, data := range entries {
		e, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZipAndFindInstaller(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "update.zip")
	writeTestZip(t, archive, map[string][]byte{
		"payload/Snishaper-1.30.1-x64Setup.exe": []byte("MZfakeinstaller"),
		"payload/readme.txt":                    []byte("notes"),
	})
	stage := filepath.Join(dir, "stage")
	if err := extractZip(archive, stage, 1<<20, 100); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	found, err := findInstaller(stage, Config{InstallerPattern: "*Setup.exe"})
	if err != nil {
		t.Fatalf("findInstaller: %v", err)
	}
	if filepath.Base(found) != "Snishaper-1.30.1-x64Setup.exe" {
		t.Fatalf("wrong installer: %s", found)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"../escape.txt", "/abs.txt", "C:evil.txt", "a/../../escape.txt"} {
		archive := filepath.Join(dir, "bad.zip")
		writeTestZip(t, archive, map[string][]byte{name: []byte("bad")})
		if err := extractZip(archive, filepath.Join(dir, "stage"), 1<<20, 100); err == nil {
			t.Fatalf("archive entry %q accepted", name)
		}
	}
}

func TestExtractZipSizeLimit(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "big.zip")
	writeTestZip(t, archive, map[string][]byte{"big.bin": make([]byte, 2048)})
	if err := extractZip(archive, filepath.Join(dir, "stage"), 1024, 100); err == nil {
		t.Fatal("size limit not enforced")
	}
}
