package app

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func writeTestZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("write entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

func TestExtractZipRoundTrip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "pkg.zip")
	writeTestZip(t, archive, map[string]string{
		"snishaper.exe":        "binary",
		"rules/config.json":    "{}",
		"config/settings.json": "{}",
	})
	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(archive, stage); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	for _, name := range []string{"snishaper.exe", "rules/config.json", "config/settings.json"} {
		if _, err := os.Stat(filepath.Join(stage, name)); err != nil {
			t.Errorf("missing extracted file %s: %v", name, err)
		}
	}
	if root := findStageRoot(stage); root != stage {
		t.Errorf("flat archive should stage in place, got %s", root)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "evil.zip")
	writeTestZip(t, archive, map[string]string{"../escaped.txt": "nope"})
	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(archive, stage); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); err == nil {
		t.Fatal("archive escaped the destination directory")
	}
}

func TestExtractZipRejectsUnsafeEntries(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(dir, "stage")
	if err := os.MkdirAll(stage, 0755); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(dir, "evil.zip")
	for _, name := range []string{
		"..\\escaped.txt",
		"a/../../escaped.txt",
		"/escaped.txt",
		"//server/share/escaped.txt",
		"C:/escaped.txt",
		"nested/..escaped.txt",
		"\x00escaped.txt",
	} {
		writeTestZip(t, archive, map[string]string{name: "nope"})
		if err := extractZip(archive, stage); err == nil {
			t.Errorf("expected entry %q to be rejected", name)
		}
		if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); err == nil {
			t.Fatal("archive escaped the destination directory")
		}
	}
}

func TestExtractZipRejectsSymlinkEntry(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "link.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	header := &zip.FileHeader{Name: "payload/link", Method: zip.Deflate}
	header.SetMode(os.ModeSymlink | 0777)
	w, err := zw.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("../../escaped.txt")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(dir, "stage")
	if err := extractZip(archive, stage); err == nil {
		t.Fatal("expected symlink entry to be rejected")
	}
	if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); err == nil {
		t.Fatal("symlink entry escaped the destination directory")
	}
}

func TestFindStageRoot(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}
	if got := findStageRoot(empty); got != "" {
		t.Errorf("empty stage = %q, want empty", got)
	}

	flat := filepath.Join(dir, "flat")
	if err := os.MkdirAll(flat, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flat, "snishaper.exe"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := findStageRoot(flat); got != flat {
		t.Errorf("flat stage = %q, want %q", got, flat)
	}

	nested := filepath.Join(dir, "nested")
	inner := filepath.Join(nested, "SniShaper")
	if err := os.MkdirAll(inner, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, "snishaper.exe"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := findStageRoot(nested); got != inner {
		t.Errorf("single-dir stage = %q, want %q", got, inner)
	}
}
