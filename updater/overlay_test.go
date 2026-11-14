//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPayloadRoot(t *testing.T) {
	flat := t.TempDir()
	if err := os.WriteFile(filepath.Join(flat, "snishaper.exe"), []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	if got := findPayloadRoot(flat); got != flat {
		t.Fatalf("flat: got %q want %q", got, flat)
	}
	empty := t.TempDir()
	if got := findPayloadRoot(empty); got != "" {
		t.Fatalf("empty: got %q want empty", got)
	}
}

func TestOverlayTree(t *testing.T) {
	src := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write("snishaper.exe", "new-exe")
	write("config/settings.json", "new-settings")
	write("rules/config.json", "new-rules")

	dst := t.TempDir()
	writeDst := func(rel, content string) {
		p := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeDst("snishaper.exe", "old-exe")
	writeDst("config/settings.json", "user-settings")
	writeDst("rules/config.json", "user-rules")
	writeDst("local-notes.txt", "keep-me")

	l := NewLogger(t.TempDir())
	defer l.Close()
	if err := overlayTree(src, dst, l); err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read("snishaper.exe"); got != "new-exe" {
		t.Errorf("snishaper.exe = %q, want new-exe", got)
	}
	if got := read("config/settings.json"); got != "user-settings" {
		t.Errorf("config/settings.json = %q, want user-settings preserved", got)
	}
	if got := read("rules/config.json"); got != "user-rules" {
		t.Errorf("rules/config.json = %q, want user-rules preserved", got)
	}
	if got := read("local-notes.txt"); got != "keep-me" {
		t.Errorf("local-notes.txt = %q, want preserved", got)
	}
}
