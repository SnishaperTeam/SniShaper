package app

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"snishaper/common"
)

// DiagnosticExportFile describes one file that a diagnostics export contains.
type DiagnosticExportFile struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	Description string `json:"description"`
}

// DiagnosticExportPreview is shown to the user BEFORE anything is written, so
// the package contents and the redaction performed on them are never a
// surprise. RedactedFields are machine keys the frontend translates.
type DiagnosticExportPreview struct {
	Files          []DiagnosticExportFile `json:"files"`
	RedactedFields []string               `json:"redacted_fields"`
}

// redactValueMask is substituted for every secret found in configs or logs.
const redactValueMask = "[REDACTED]"

// sensitiveJSONKeys matches config keys whose value is a credential. Matched
// case-insensitively as a substring: api_key, apikey, token, access_token,
// secret, password, authorization.
var sensitiveJSONKeys = regexp.MustCompile(`(?i)api_?key|token|secret|password|authorization`)

// urlCredentialRe matches userinfo in URLs (scheme://user:pass@host) so DoH
// URLs or upstream addresses carrying inline credentials are masked.
var urlCredentialRe = regexp.MustCompile(`(?i)(://[^/@:\s]+):([^/@\s]+)@`)

// secretQueryRe matches credential-looking query parameters in free text
// (?api_key=...&token=...).
var secretQueryRe = regexp.MustCompile(`(?i)((?:api_?key|access_?token|token|secret|password|passwd)=)[^&\s]+`)

// bearerRe matches Authorization-style bearer tokens in logs.
var bearerRe = regexp.MustCompile(`(?i)(bearer\s+)\S+`)

// redactSensitiveText masks credentials in free-form text (logs): URL
// userinfo, secret query parameters and bearer tokens.
func redactSensitiveText(s string) string {
	s = urlCredentialRe.ReplaceAllString(s, "://"+redactValueMask+"@")
	s = secretQueryRe.ReplaceAllString(s, "$1"+redactValueMask)
	s = bearerRe.ReplaceAllString(s, "$1"+redactValueMask)
	return s
}

// redactJSON walks a parsed JSON value and masks every credential: values
// under sensitive keys, and URL userinfo inside strings. The walk mutates the
// parsed copy only — the on-disk config is never touched.
func redactJSON(v interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if sensitiveJSONKeys.MatchString(k) {
				if s, ok := val.(string); ok && s != "" {
					t[k] = redactValueMask
					continue
				}
			}
			if s, ok := val.(string); ok {
				t[k] = redactSensitiveText(s)
				continue
			}
			redactJSON(val)
		}
	case []interface{}:
		for i, val := range t {
			if s, ok := val.(string); ok {
				t[i] = redactSensitiveText(s)
				continue
			}
			redactJSON(val)
		}
	}
}

// sanitizeJSONBytes parses raw JSON config, redacts credentials and returns
// the sanitized bytes. Unparseable input comes back redacted as text so a
// corrupt config still yields a safe (not silent) export entry.
func sanitizeJSONBytes(raw []byte) []byte {
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return []byte(redactSensitiveText(string(raw)))
	}
	redactJSON(parsed)
	out, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return []byte(redactSensitiveText(string(raw)))
	}
	return out
}

// diagnosticsFileSet returns the (name, description) pairs of the package in
// a stable order; the same list backs the preview and the actual export.
func diagnosticsFileSet() [][2]string {
	return [][2]string{
		{"diagnostics.json", "version, platform, network state, self-check"},
		{"settings.json", "app settings (sanitized)"},
		{"rules.json", "site group rules (sanitized)"},
		{"logs.txt", "recent application logs (sanitized)"},
	}
}

// PreviewDiagnosticsExport lists what ExportDiagnostics would write and which
// sensitive fields get redacted, without writing anything.
func (a *App) PreviewDiagnosticsExport() DiagnosticExportPreview {
	preview := DiagnosticExportPreview{
		Files:          []DiagnosticExportFile{},
		RedactedFields: []string{"api_key", "url_credentials", "query_token", "bearer"},
	}

	settingsPath, rulesPath := a.diagnosticConfigPaths()
	for _, entry := range diagnosticsFileSet() {
		item := DiagnosticExportFile{Name: entry[0], Description: entry[1]}
		switch entry[0] {
		case "settings.json":
			if info, err := os.Stat(settingsPath); err == nil {
				item.Size = info.Size()
			}
		case "rules.json":
			if info, err := os.Stat(rulesPath); err == nil {
				item.Size = info.Size()
			}
		case "logs.txt":
			if a.logBuffer != nil {
				item.Size = int64(len(strings.Join(a.logBuffer.Snapshot(500), "\n")))
			}
		case "diagnostics.json":
			item.Size = 2048 // snapshot marshals to roughly this size
		}
		preview.Files = append(preview.Files, item)
	}
	return preview
}

// diagnosticConfigPaths resolves the on-disk config locations from the
// executable directory, the same way NewApp does.
func (a *App) diagnosticConfigPaths() (settingsPath, rulesPath string) {
	execPath, err := os.Executable()
	if err != nil {
		return "", ""
	}
	execDir := filepath.Dir(execPath)
	return common.ConfigSettingsPath(execDir), common.ConfigRulesPath(execDir)
}

// ExportDiagnostics writes the diagnostic package as a zip next to the log
// files and returns its path. It only reads runtime state and config files;
// a failure here never touches the running proxy.
func (a *App) ExportDiagnostics() (string, error) {
	settingsPath, rulesPath := a.diagnosticConfigPaths()

	type diagPackage struct {
		GeneratedAt  string             `json:"generated_at"`
		Version      string             `json:"version"`
		Diagnostics  ProxyDiagnostics   `json:"diagnostics"`
		SelfCheck    ProxySelfCheckResult `json:"self_check"`
		Files        []DiagnosticExportFile `json:"files"`
	}

	files := a.PreviewDiagnosticsExport().Files
	pkg := diagPackage{
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     VersionString(),
		Diagnostics: a.GetProxyDiagnostics(),
		SelfCheck:   a.ProxySelfCheck(),
		Files:       files,
	}
	diagBytes, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal diagnostics: %w", err)
	}

	logBytes := []byte("")
	if a.logBuffer != nil {
		logBytes = []byte(redactSensitiveText(strings.Join(a.logBuffer.Snapshot(500), "\n")))
	}

	settingsBytes := []byte("(settings file not found)")
	if raw, err := os.ReadFile(settingsPath); err == nil {
		settingsBytes = sanitizeJSONBytes(raw)
	}
	rulesBytes := []byte("(rules file not found)")
	if raw, err := os.ReadFile(rulesPath); err == nil {
		rulesBytes = sanitizeJSONBytes(raw)
	}

	name := "snishaper-diagnostics-" + time.Now().Format("20060102-150405") + ".zip"
	outPath := filepath.Join(a.logDir, name)
	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("create diagnostics zip: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	entries := []struct {
		name string
		data []byte
	}{
		{"diagnostics.json", diagBytes},
		{"settings.json", settingsBytes},
		{"rules.json", rulesBytes},
		{"logs.txt", logBytes},
	}
	for _, e := range entries {
		w, err := zw.Create(e.name)
		if err != nil {
			return "", fmt.Errorf("zip entry %s: %w", e.name, err)
		}
		if _, err := w.Write(e.data); err != nil {
			return "", fmt.Errorf("zip write %s: %w", e.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return "", fmt.Errorf("close diagnostics zip: %w", err)
	}

	a.appendLog("[diagnostics] exported " + name)
	return outPath, nil
}

// RevealDiagnosticsExport opens the file manager with the exported zip
// selected. Errors are logged, not returned: the export itself already
// succeeded when this is called.
func (a *App) RevealDiagnosticsExport(path string) {
	// Only reveal files this app wrote into its own log dir.
	if filepath.Dir(path) != a.logDir || filepath.Base(path) != path {
		return
	}
	if err := common.RevealPath(path); err != nil {
		a.appendLog("[diagnostics] failed to reveal export: " + err.Error())
	}
}
