package app

// Multiple configuration profiles.
//
// A profile is a named snapshot of the live configuration (settings.json +
// rules/config.json) stored under <UserConfigDir>/profiles/<name>/. The live
// files are the single runtime source of truth and are never moved — profiles
// only copy into and out of them, so installations that never touch profiles
// behave exactly as before.
//
// Active-profile marker: profiles/_active.json records which profile the live
// files currently mirror. Switching away from an active profile snapshots the
// live files back into it first, so in-profile edits survive a switch cycle.
//
// Switch semantics (documented in the UI): the proxy is stopped first —
// active connections are dropped — then the target profile's files replace
// the live ones and both the in-app rule manager and the core subprocess
// reload. The frontend reloads after a successful switch.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"snishaper/common"
	corelog "github.com/SnishaperTeam/snishaper-core/log"
	"github.com/SnishaperTeam/snishaper-core/proxy"
)

// ProfileInfo describes one stored profile.
type ProfileInfo struct {
	Name      string `json:"name"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Active    bool   `json:"active"`
}

// ProfileImportResult reports validation of an imported profile bundle.
type ProfileImportResult struct {
	Name     string   `json:"name"`
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

// profileBundle is the export/import interchange format (one JSON file).
type profileBundle struct {
	Format     string          `json:"format"` // "snishaper-profile"
	Version    int             `json:"version"`
	Name       string          `json:"name"`
	ExportedAt string          `json:"exported_at"`
	Settings   json.RawMessage `json:"settings"`
	Rules      json.RawMessage `json:"rules"`
}

const profileBundleFormat = "snishaper-profile"
const profileBundleVersion = 1

var profileNameRe = regexp.MustCompile(`^[\p{Han}\p{Latin}0-9_\-][\p{Han}\p{Latin}0-9_\- ]{0,63}$`)

func profilesDir() string {
	return common.UserConfigPath("profiles")
}

func profileDir(name string) string {
	return filepath.Join(profilesDir(), name)
}

func activeProfileMarkerPath() string {
	return filepath.Join(profilesDir(), "_active.json")
}

// validateProfileName rejects reserved, empty, overlong or path-hostile names.
func validateProfileName(name string) error {
	if !profileNameRe.MatchString(name) {
		return fmt.Errorf("invalid profile name (allowed: letters, digits, CJK, space, _ and -, max 64 chars)")
	}
	if strings.HasPrefix(name, "_") {
		return fmt.Errorf("profile names starting with '_' are reserved")
	}
	return nil
}

// GetActiveProfile returns the profile the live config currently mirrors
// ("" when none — e.g. the user never used profiles).
func (a *App) GetActiveProfile() string {
	data, err := os.ReadFile(activeProfileMarkerPath())
	if err != nil {
		return ""
	}
	var marker struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &marker) != nil {
		return ""
	}
	if marker.Name == "" {
		return ""
	}
	if _, err := os.Stat(profileDir(marker.Name)); err != nil {
		return "" // profile was removed; the marker is stale
	}
	return marker.Name
}

func writeActiveProfile(name string) {
	_ = os.MkdirAll(profilesDir(), 0755)
	data, _ := json.Marshal(map[string]string{
		"name":        name,
		"switched_at": time.Now().Format(time.RFC3339),
	})
	_ = os.WriteFile(activeProfileMarkerPath(), data, 0644)
}

// copyFile copies src to dst atomically enough for config files (temp file +
// rename in the destination directory).
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

// snapshotLiveInto copies the current live settings and rules into dir.
func (a *App) snapshotLiveInto(dir string) error {
	if err := copyFile(a.settingsPath, filepath.Join(dir, "settings.json")); err != nil {
		return err
	}
	return copyFile(a.rulesPath, filepath.Join(dir, "rules.json"))
}

// ListProfiles returns all stored profiles, newest update first.
func (a *App) ListProfiles() []ProfileInfo {
	active := a.GetActiveProfile()
	entries, err := os.ReadDir(profilesDir())
	if err != nil {
		return []ProfileInfo{}
	}
	var out []ProfileInfo
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), "_") {
			continue
		}
		info, err := e.Info()
		updated := ""
		if err == nil {
			updated = info.ModTime().Format(time.RFC3339)
		}
		out = append(out, ProfileInfo{
			Name:      e.Name(),
			UpdatedAt: updated,
			Active:    e.Name() == active,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

// CreateProfile snapshots the current live configuration as a new profile.
func (a *App) CreateProfile(name string) (ProfileInfo, error) {
	if err := validateProfileName(name); err != nil {
		return ProfileInfo{}, err
	}
	dir := profileDir(name)
	if _, err := os.Stat(dir); err == nil {
		return ProfileInfo{}, fmt.Errorf("profile %q already exists", name)
	}
	if err := a.snapshotLiveInto(dir); err != nil {
		return ProfileInfo{}, fmt.Errorf("snapshot live config: %w", err)
	}
	a.appendLog(fmt.Sprintf("[profiles] created profile %q from current configuration", name))
	return ProfileInfo{Name: name, Active: a.GetActiveProfile() == name}, nil
}

// RenameProfile renames a stored profile, keeping the active marker in sync.
func (a *App) RenameProfile(oldName, newName string) error {
	if err := validateProfileName(newName); err != nil {
		return err
	}
	oldDir := profileDir(oldName)
	if _, err := os.Stat(oldDir); err != nil {
		return fmt.Errorf("profile %q does not exist", oldName)
	}
	if _, err := os.Stat(profileDir(newName)); err == nil {
		return fmt.Errorf("profile %q already exists", newName)
	}
	if err := os.Rename(oldDir, profileDir(newName)); err != nil {
		return err
	}
	if a.GetActiveProfile() == oldName {
		writeActiveProfile(newName)
	}
	a.appendLog(fmt.Sprintf("[profiles] renamed profile %q to %q", oldName, newName))
	return nil
}

// DuplicateProfile copies an existing profile under a new name.
func (a *App) DuplicateProfile(srcName, dstName string) error {
	if err := validateProfileName(dstName); err != nil {
		return err
	}
	srcDir := profileDir(srcName)
	if _, err := os.Stat(srcDir); err != nil {
		return fmt.Errorf("profile %q does not exist", srcName)
	}
	if _, err := os.Stat(profileDir(dstName)); err == nil {
		return fmt.Errorf("profile %q already exists", dstName)
	}
	if err := snapshotDirInto(srcDir, profileDir(dstName)); err != nil {
		return err
	}
	a.appendLog(fmt.Sprintf("[profiles] duplicated profile %q as %q", srcName, dstName))
	return nil
}

func snapshotDirInto(srcDir, dstDir string) error {
	for _, f := range []string{"settings.json", "rules.json"} {
		if err := copyFile(filepath.Join(srcDir, f), filepath.Join(dstDir, f)); err != nil {
			return err
		}
	}
	return nil
}

// SwitchProfile activates a stored profile. Safe behavior while connections
// are active: the proxy (and with it TUN/system proxy and every live
// connection) is stopped first; the outgoing profile's edits are snapshotted
// back so nothing is lost; the live files are replaced from the target
// profile and both rule managers (app + core subprocess) reload.
func (a *App) SwitchProfile(name string) error {
	dir := profileDir(name)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("profile %q does not exist", name)
	}
	active := a.GetActiveProfile()
	if active == name {
		return fmt.Errorf("profile %q is already active", name)
	}

	a.appendLog(fmt.Sprintf("[profiles] switching to profile %q", name))

	// Stop the proxy first: profile files are the runtime source of truth
	// and must never change under a running engine.
	if a.IsProxyRunning() {
		if err := a.StopProxy(); err != nil {
			return fmt.Errorf("cannot switch profiles: stopping the proxy failed: %w", err)
		}
	}

	// Preserve edits made while the outgoing profile was active.
	if active != "" {
		if err := a.snapshotLiveInto(profileDir(active)); err != nil {
			a.appendLog("[profiles] warning: failed to snapshot outgoing profile: " + err.Error())
		}
	}

	for _, f := range []struct{ src, dst string }{
		{filepath.Join(dir, "settings.json"), a.settingsPath},
		{filepath.Join(dir, "rules.json"), a.rulesPath},
	} {
		if err := copyFile(f.src, f.dst); err != nil {
			return fmt.Errorf("apply profile file %s: %w", f.src, err)
		}
	}

	if err := a.ruleManager.LoadConfig(); err != nil {
		return fmt.Errorf("reload configuration: %w", err)
	}
	if port := a.ruleManager.GetListenPort(); port != "" {
		_ = a.proxyServer.SetListenAddr("127.0.0.1:" + port)
	}

	writeActiveProfile(name)
	if a.core != nil {
		a.core.ReloadIfRunning()
	}
	a.UpdateTrayMenu()
	a.emitFrontendState()
	a.appendLog(fmt.Sprintf("[profiles] switched to profile %q (proxy stopped, start it when ready)", name))
	return nil
}

// ExportProfile writes a profile as a single portable JSON bundle and returns
// its path. Bundles are placed under the log directory's profile-exports/
// so the export never writes into arbitrary locations.
func (a *App) ExportProfile(name string) (string, error) {
	dir := profileDir(name)
	if _, err := os.Stat(dir); err != nil {
		return "", fmt.Errorf("profile %q does not exist", name)
	}
	settings, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		return "", err
	}
	rules, err := os.ReadFile(filepath.Join(dir, "rules.json"))
	if err != nil {
		return "", err
	}
	bundle := profileBundle{
		Format:     profileBundleFormat,
		Version:    profileBundleVersion,
		Name:       name,
		ExportedAt: time.Now().Format(time.RFC3339),
		Settings:   json.RawMessage(settings),
		Rules:      json.RawMessage(rules),
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	outDir := filepath.Join(a.logDir, "profile-exports")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}
	out := filepath.Join(outDir, fmt.Sprintf("%s-%s.json", name, time.Now().Format("20060102-150405")))
	if err := os.WriteFile(out, data, 0644); err != nil {
		return "", err
	}
	a.appendLog(fmt.Sprintf("[profiles] exported profile %q to %s", name, out))
	return out, nil
}

// RevealProfileExport opens the file manager with an exported bundle
// selected. Only files this app wrote into its own profile-exports dir are
// revealed; errors are logged, not returned.
func (a *App) RevealProfileExport(path string) {
	exportsDir := filepath.Join(a.logDir, "profile-exports")
	if !strings.HasPrefix(filepath.Clean(path), exportsDir+string(filepath.Separator)) {
		return
	}
	if err := common.RevealPath(path); err != nil {
		a.appendLog("[profiles] failed to reveal export: " + err.Error())
	}
}

// ImportProfile validates a profile bundle and, when valid, stores it as a
// new profile. Validation runs the bundle through a real RuleManager load in
// a temp directory — the same pipeline as live matching — so invalid or
// conflicting settings are rejected with the engine's own explanation
// instead of a hand-rolled subset of checks.
func (a *App) ImportProfile(bundleJSON string) (ProfileImportResult, error) {
	result := ProfileImportResult{Warnings: []string{}, Errors: []string{}}
	var bundle profileBundle
	if err := json.Unmarshal([]byte(bundleJSON), &bundle); err != nil {
		return result, fmt.Errorf("not a valid profile bundle: %w", err)
	}
	if bundle.Format != profileBundleFormat {
		return result, fmt.Errorf("not a SniShaper profile bundle (format %q)", bundle.Format)
	}
	if err := validateProfileName(bundle.Name); err != nil {
		return result, err
	}
	if len(bundle.Settings) == 0 || len(bundle.Rules) == 0 {
		return result, fmt.Errorf("bundle is missing settings or rules")
	}
	if _, err := os.Stat(profileDir(bundle.Name)); err == nil {
		return result, fmt.Errorf("profile %q already exists — rename it first", bundle.Name)
	}
	result.Name = bundle.Name

	// Validate through the real load pipeline in a temp dir.
	tmp, err := os.MkdirTemp("", "snishaper-profile-import-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(tmp)
	tmpSettings := filepath.Join(tmp, "settings.json")
	tmpRules := filepath.Join(tmp, "rules.json")
	if err := os.WriteFile(tmpSettings, bundle.Settings, 0644); err != nil {
		return result, err
	}
	if err := os.WriteFile(tmpRules, bundle.Rules, 0644); err != nil {
		return result, err
	}
	rm := proxy.NewRuleManager(tmpSettings, tmpRules, proxy.WithLogger(corelog.Std))
	report, err := rm.LoadConfigWithMigration()
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, fmt.Errorf("invalid profile: %s", strings.Join(result.Errors, "; "))
	}
	result.Warnings = report.Warnings
	result.Errors = report.Errors
	if len(report.Errors) > 0 {
		return result, fmt.Errorf("invalid profile: %s", strings.Join(report.Errors, "; "))
	}

	if err := snapshotDirInto(tmp, profileDir(bundle.Name)); err != nil {
		return result, err
	}
	result.Valid = true
	a.appendLog(fmt.Sprintf("[profiles] imported profile %q (%d warning(s))", bundle.Name, len(report.Warnings)))
	return result, nil
}
