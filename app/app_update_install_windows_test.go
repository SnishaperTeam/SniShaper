//go:build windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPortableUpdaterInDir(t *testing.T) {
	dir := t.TempDir()
	if got := portableUpdaterInDir(dir); got != "" {
		t.Fatalf("empty dir should not resolve an updater, got %s", got)
	}
	exe := filepath.Join(dir, portableUpdaterName)
	if err := os.WriteFile(exe, []byte("MZ"), 0644); err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(exe)
	got, _ := filepath.Abs(portableUpdaterInDir(dir))
	if got != want {
		t.Fatalf("portableUpdaterInDir = %s, want %s", got, want)
	}
	if err := os.Remove(exe); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(exe, 0755); err != nil {
		t.Fatal(err)
	}
	if got := portableUpdaterInDir(dir); got != "" {
		t.Fatalf("directory must not be treated as updater exe: %s", got)
	}
}

func TestUpdaterArgs(t *testing.T) {
	got := updaterArgs(`C:\Program Files\Snishaper\updater.json`)
	want := `-config "C:\Program Files\Snishaper\updater.json"`
	if got != want {
		t.Fatalf("updaterArgs = %s, want %s", got, want)
	}
}

func TestInstallUpdatePathHasNoExternalCommands(t *testing.T) {
	data, err := os.ReadFile("app_update_install_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.ToLower(string(data))
	for _, banned := range []string{"powershell", "pwsh", "cmd.exe", "cmd /c", "tar -xf", "7z.exe", "apply-update.ps1", "os/exec", "exec.command"} {
		if strings.Contains(src, banned) {
			t.Errorf("update install path references external command %q", banned)
		}
	}
}

func TestInstallUpdateAssetRejectsArchiveWithoutUpdater(t *testing.T) {
	for _, name := range []string{"update.zip", "update.7z", "payload.bin"} {
		got := archiveInstallError(name)
		if !strings.Contains(got, "portable updater not found") {
			t.Errorf("%s: unexpected message %q", name, got)
		}
	}
	if got := archiveInstallError("Snishaper-Setup.exe"); got != "" {
		t.Errorf("exe fallback should be allowed, got %q", got)
	}
}
