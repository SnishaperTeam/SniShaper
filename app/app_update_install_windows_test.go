//go:build windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPsQuoteSingle(t *testing.T) {
	if got := psQuoteSingle(`C:\Users\O'Brien\app`); got != `C:\Users\O''Brien\app` {
		t.Fatalf("unexpected escaping: %s", got)
	}
	if got := psQuoteSingle(`C:\plain\path`); got != `C:\plain\path` {
		t.Fatalf("plain path should stay unchanged, got: %s", got)
	}
}

func TestBuildUpdateScriptEscapesQuotes(t *testing.T) {
	script := buildUpdateScript(`C:\tmp\a'b.7z`, `C:\tmp\stage`, `C:\Users\x'y`, `C:\tmp\base`)
	// 未转义的单引号会截断 PowerShell 字符串，必须翻倍转义
	if strings.Contains(script, "a'b") || strings.Contains(script, "x'y") {
		t.Fatal("unescaped single quote in update script")
	}
	if !strings.Contains(script, "a''b") || !strings.Contains(script, "x''y") {
		t.Fatal("expected doubled single quotes in update script")
	}
}

func TestBuildUpdateScriptLegacyArchiveExtraction(t *testing.T) {
	script := buildUpdateScript(`C:\tmp\pkg.7z`, `C:\tmp\stage`, `C:\Users\x`, `C:\tmp\base`)
	if !strings.Contains(script, "tar -xf") || !strings.Contains(script, "7z.exe") {
		t.Fatal("legacy 7z script must keep the tar/7z extraction path")
	}
}

func TestBuildUpdateScriptPreExtractedPayload(t *testing.T) {
	script := buildUpdateScript("", `C:\tmp\stage`, `C:\Users\x`, `C:\tmp\base`)
	// The legacy extraction branch stays in the rendered script; an empty
	// $archive is what makes the updater skip it for a zip payload.
	if !strings.Contains(script, `$archive = ''`) {
		t.Fatal("empty legacy archive must render as an empty PowerShell string")
	}
	if !strings.Contains(script, "更新包已解压") {
		t.Fatal("expected the already-extracted branch in the update script")
	}
	if !strings.Contains(script, "Test-Path $stagedExe") {
		t.Fatal("pre-extracted payload must be verified before the swap")
	}
}

// TestBuildUpdateScriptIsValidPowerShell parses both renderings with the real
// PowerShell parser, so a typo in the template cannot reach a release.
func TestBuildUpdateScriptIsValidPowerShell(t *testing.T) {
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Skip("pwsh not installed")
	}
	cases := map[string]string{
		"zip":    buildUpdateScript("", `C:\tmp\stage`, `C:\Users\x`, `C:\tmp\base`),
		"legacy": buildUpdateScript(`C:\tmp\pkg.7z`, `C:\tmp\stage`, `C:\Users\x`, `C:\tmp\base`),
	}
	for name, script := range cases {
		path := filepath.Join(t.TempDir(), "apply-update.ps1")
		bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(script)...)
		if err := os.WriteFile(path, bom, 0644); err != nil {
			t.Fatalf("%s: write script: %v", name, err)
		}
		check := "$e=$null; [void][System.Management.Automation.Language.Parser]::ParseFile('" +
			strings.ReplaceAll(path, "'", "''") + "',[ref]$null,[ref]$e); if ($e) { $e | ForEach-Object { $_.Message }; exit 1 }"
		out, err := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", check).CombinedOutput()
		if err != nil {
			t.Fatalf("%s: generated script is not valid PowerShell: %v\n%s", name, err, out)
		}
	}
}
