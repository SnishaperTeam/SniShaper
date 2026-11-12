//go:build windows

package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type Config struct {
	ManifestURL        string   `json:"manifest_url"`
	MirrorURLs         []string `json:"mirror_urls"`
	PublicKey          string   `json:"public_key"`
	Channel            string   `json:"channel"`
	Arch               string   `json:"arch"`
	InstallerPattern   string   `json:"installer_pattern"`
	InstallerName      string   `json:"installer_name"`
	ProcessNames       []string `json:"process_names"`
	AppID              string   `json:"app_id"`
	InstallDir         string   `json:"install_dir"`
	LocalVersion       string   `json:"local_version"`
	VerifyAuthenticode bool     `json:"verify_authenticode"`
	RestartAfter       bool     `json:"restart_after_install"`
	HTTPTimeoutMinutes int      `json:"http_timeout_minutes"`
	MaxRetries         int      `json:"max_retries"`
	MaxExtractBytes    int64    `json:"max_extract_bytes"`
}

type PackageEntry struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type InstallerEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type UpdaterEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

type Manifest struct {
	Version     string         `json:"version"`
	Channel     string         `json:"channel"`
	Notes       string         `json:"notes"`
	Package     PackageEntry   `json:"package"`
	Installer   InstallerEntry `json:"installer"`
	Updater     UpdaterEntry   `json:"updater"`
	PublishedAt string         `json:"published_at"`
}

type ResumeState struct {
	ConfigPath string `json:"config_path"`
	BaseDir    string `json:"base_dir"`
	InstallDir string `json:"install_dir"`
	LogPath    string `json:"log_path"`
	WorkDir    string `json:"work_dir"`
}

type installedInfo struct {
	Version string
	Dir     string
	Name    string
}

func loadConfig(path string) (Config, error) {
	c := Config{
		Channel:            "stable",
		Arch:               "auto",
		InstallerPattern:   "*Setup.exe",
		ProcessNames:       []string{"snishaper.exe"},
		AppID:              "{3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D4}",
		HTTPTimeoutMinutes: 30,
		MaxRetries:         3,
		MaxExtractBytes:    1 << 30,
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	if err := c.validate(); err != nil {
		return c, err
	}
	return c, nil
}

func (c Config) validate() error {
	if !strings.HasPrefix(strings.ToLower(c.ManifestURL), "https://") {
		return fmt.Errorf("manifest_url must be an https URL")
	}
	if strings.TrimSpace(c.AppID) == "" {
		return fmt.Errorf("app_id is required")
	}
	if c.PublicKey != "" {
		if _, err := decodePublicKey(c.PublicKey); err != nil {
			return err
		}
	}
	if c.HTTPTimeoutMinutes <= 0 {
		return fmt.Errorf("http_timeout_minutes must be positive")
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max_retries must not be negative")
	}
	if c.MaxExtractBytes <= 0 {
		return fmt.Errorf("max_extract_bytes must be positive")
	}
	return nil
}

func resolveArch(c Config) string {
	a := strings.ToLower(strings.TrimSpace(c.Arch))
	if a != "" && a != "auto" {
		return a
	}
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	case "arm64":
		return "arm64"
	}
	return runtime.GOARCH
}

func normalizeAppID(id string) string {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "{") {
		id = "{" + id
	}
	if !strings.HasSuffix(id, "}") {
		id = id + "}"
	}
	return id
}

func readInstalled(c Config) installedInfo {
	sub := `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\` + normalizeAppID(c.AppID) + `_is1`
	views := []struct {
		root  registry.Key
		flags uint32
	}{
		{registry.LOCAL_MACHINE, windows.KEY_WOW64_64KEY},
		{registry.LOCAL_MACHINE, windows.KEY_WOW64_32KEY},
		{registry.CURRENT_USER, windows.KEY_WOW64_64KEY},
		{registry.CURRENT_USER, windows.KEY_WOW64_32KEY},
	}
	for _, v := range views {
		k, err := registry.OpenKey(v.root, sub, registry.QUERY_VALUE|v.flags)
		if err != nil {
			continue
		}
		get := func(name string) string {
			s, _, e := k.GetStringValue(name)
			if e != nil {
				return ""
			}
			return s
		}
		info := installedInfo{
			Version: get("DisplayVersion"),
			Dir:     get("InstallLocation"),
			Name:    get("DisplayName"),
		}
		_ = k.Close()
		if info.Version != "" || info.Dir != "" {
			return info
		}
	}
	return installedInfo{}
}

func localVersion(c Config, baseDir string, installed installedInfo) string {
	if v := strings.TrimSpace(c.LocalVersion); v != "" && !strings.EqualFold(v, "auto") {
		return v
	}
	if v := strings.TrimSpace(installed.Version); v != "" {
		return v
	}
	if v, err := manifestVersion(filepath.Join(baseDir, "Package.appxmanifest")); err == nil && v != "" {
		return v
	}
	if b, err := os.ReadFile(filepath.Join(baseDir, "version.txt")); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" {
			return v
		}
	}
	return "0.0.0.0"
}

func manifestVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var pkg struct {
		RelVersion string `xml:"http://schemas.snishaper.dev/release Version"`
		RelChannel string `xml:"http://schemas.snishaper.dev/release ReleaseChannel"`
		Identity   struct {
			Version string `xml:"Version,attr"`
		} `xml:"Identity"`
	}
	if err := xml.Unmarshal(data, &pkg); err != nil {
		return "", err
	}
	if v := strings.TrimSpace(pkg.RelVersion); v != "" {
		if ch := strings.TrimSpace(pkg.RelChannel); ch != "" && channelRank(ch) < channelRank("stable") {
			return v + "-" + ch, nil
		}
		return v, nil
	}
	parts := strings.Split(pkg.Identity.Version, ".")
	for len(parts) > 1 && parts[len(parts)-1] == "0" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) > 0 && parts[0] != "" {
		return strings.Join(parts, "."), nil
	}
	return "", fmt.Errorf("no version in manifest")
}

func parseVersion(v string) ([]int, []string) {
	v = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(v), "v"), "V")
	var pre []string
	if i := strings.Index(v, "-"); i >= 0 {
		pre = strings.Split(v[i+1:], ".")
		v = v[:i]
	}
	nums := []int{}
	for _, p := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(strings.TrimSpace(p))
		nums = append(nums, n)
	}
	return nums, pre
}

func channelRank(ch string) int {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(ch, ".", 2)[0])) {
	case "alpha":
		return 0
	case "beta":
		return 1
	case "rc":
		return 2
	default:
		return 3
	}
}

func channelOf(v string) string {
	i := strings.Index(v, "-")
	if i < 0 {
		return "stable"
	}
	return v[i+1:]
}

func preNum(pre []string) int {
	for _, p := range pre {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	return 0
}

func compareVersions(current, target string) int {
	cn, cp := parseVersion(current)
	tn, tp := parseVersion(target)
	maxLen := len(cn)
	if len(tn) > maxLen {
		maxLen = len(tn)
	}
	for i := 0; i < maxLen; i++ {
		var a, b int
		if i < len(cn) {
			a = cn[i]
		}
		if i < len(tn) {
			b = tn[i]
		}
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
	}
	cr := channelRank(channelOf(current))
	tr := channelRank(channelOf(target))
	if tr > cr {
		return -1
	}
	if tr < cr {
		return 1
	}
	a, b := preNum(cp), preNum(tp)
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func versionEqual(a, b string) bool {
	return compareVersions(a, b) == 0
}

func loadResumeState(path string) (ResumeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ResumeState{}, err
	}
	var s ResumeState
	if err := json.Unmarshal(data, &s); err != nil {
		return ResumeState{}, err
	}
	return s, nil
}

func writeResumeState(path string, s ResumeState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
