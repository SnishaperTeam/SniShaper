package app

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows/registry"
)

const (
	githubRepo      = "SniShaper/SniShaper"
	githubAPIBase   = "https://api.github.com/repos/" + githubRepo
	githubProxyBase = "https://gh.llkk.cc/"
	updateUserAgent = "SniShaper-Update/1.0"
	relNS           = "http://schemas.snishaper.dev/release"
)

var buildVersion string

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Name       string        `json:"name"`
	Prerelease bool          `json:"prerelease"`
	Published  string        `json:"published_at"`
	Body       string        `json:"body"`
	Assets     []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
}

var validUpdateChannels = map[string]string{
	"stable": "",
	"rc":     "rc",
	"beta":   "beta",
	"alpha":  "alpha",
}

func (a *App) GetUpdateChannel() string {
	return a.ruleManager.GetUpdateChannel()
}

func (a *App) SetUpdateChannel(channel string) error {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if _, ok := validUpdateChannels[channel]; !ok {
		return fmt.Errorf("invalid update channel: %s", channel)
	}
	a.appendLog("[update] Channel set to: " + channel)
	return a.ruleManager.SetUpdateChannel(channel)
}

func (a *App) GetReleaseChannel() string {
	if ch, err := manifestChannel(); err == nil && ch != "" {
		return normalizeReleaseChannel(ch)
	}
	if v, err := manifestVersionFull(); err == nil && v != "" {
		return normalizeReleaseChannel(channelFromTag(v))
	}
	return "stable"
}

func (a *App) GetCurrentVersionFull() string {
	return a.GetAppVersion()
}

func normalizeReleaseChannel(ch string) string {
	v := strings.ToLower(strings.TrimSpace(ch))
	if i := strings.Index(v, "."); i > 0 {
		v = v[:i]
	}
	switch v {
	case "alpha":
		return "alpha"
	case "beta":
		return "beta"
	case "rc", "rc1", "releasecandidate", "release-candidate", "candidate":
		return "rc"
	case "stable", "release", "official", "final":
		return "stable"
	default:
		return "stable"
	}
}

func manifestChannel() (string, error) {
	dir := filepath.Dir(os.Args[0])
	var lastErr error
	for {
		for _, name := range []string{"Package.appxmanifest", "AppxManifest.xml"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var pkg struct {
				Channel string `xml:"http://schemas.snishaper.dev/release ReleaseChannel"`
			}
			if err := xml.Unmarshal(data, &pkg); err != nil {
				lastErr = err
				continue
			}
			if strings.TrimSpace(pkg.Channel) != "" {
				return strings.TrimSpace(pkg.Channel), nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("manifest not found")
}

func manifestVersionFull() (string, error) {
	dir := filepath.Dir(os.Args[0])
	var lastErr error
	identityFallback := ""
	for {
		for _, name := range []string{"Package.appxmanifest", "AppxManifest.xml"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var pkg struct {
				RelVersion string `xml:"http://schemas.snishaper.dev/release Version"`
				RelChannel string `xml:"http://schemas.snishaper.dev/release ReleaseChannel"`
				Identity   struct {
					Version string `xml:"Version,attr"`
				} `xml:"Identity"`
			}
			if err := xml.Unmarshal(data, &pkg); err != nil {
				lastErr = err
				continue
			}
			rv := strings.TrimSpace(pkg.RelVersion)
			rc := strings.TrimSpace(pkg.RelChannel)
			if rv != "" {
				if rc != "" && normalizeReleaseChannel(rc) != "stable" {
					return rv + "-" + rc, nil
				}
				return rv, nil
			}
			if identityFallback == "" && pkg.Identity.Version != "" {
				parts := strings.Split(pkg.Identity.Version, ".")
				for len(parts) > 1 && parts[len(parts)-1] == "0" {
					parts = parts[:len(parts)-1]
				}
				identityFallback = strings.Join(parts, ".")
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if identityFallback != "" {
		return identityFallback, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("manifest not found")
}

func (a *App) CheckUpdate() CheckUpdateResult {
	channel := a.GetUpdateChannel()
	releases, err := a.fetchGitHubReleases()
	if err != nil {
		a.appendLog("[update] Failed to fetch releases: " + err.Error())
		return CheckUpdateResult{
			HasUpdate:   false,
			Message:     "check_failed",
			ErrorDetail: classifyUpdateError(err),
		}
	}

	rel := resolveChannelRelease(releases, channel)
	if rel == nil {
		a.appendLog("[update] No release found for channel " + channel)
		return CheckUpdateResult{
			HasUpdate: false,
			Message:   "no_release_found",
		}
	}

	latestVersion := strings.TrimPrefix(rel.TagName, "v")
	currentFull := a.GetCurrentVersionFull()
	currentChannel := a.GetReleaseChannel()
	targetChannel := channelFromTag(rel.TagName)
	a.appendLog(fmt.Sprintf("[update] Channel=%s Current=%s(%s) Latest=%s(%s) tag=%s", channel, currentFull, currentChannel, latestVersion, targetChannel, rel.TagName))

	switch compareReleaseVersions(currentFull, currentChannel, latestVersion, targetChannel) {
	case -1:
		assets := filterUpdateAssets(rel.Assets)
		result := CheckUpdateResult{
			HasUpdate:     true,
			LatestVersion: latestVersion,
			Channel:       channel,
			ReleaseName:   rel.Name,
			ReleaseNotes:  rel.Body,
			Assets:        assets,
			Message:       "update_available",
		}
		if len(assets) > 0 {
			result.DownloadURL = assets[0].DownloadURL
		}
		return result
	case 0:
		return CheckUpdateResult{
			HasUpdate:     false,
			LatestVersion: latestVersion,
			Channel:       channel,
			Message:       "up_to_date",
		}
	default:
		return CheckUpdateResult{
			HasUpdate:     false,
			LatestVersion: latestVersion,
			Channel:       channel,
			Message:       "dev_version",
		}
	}
}

func (a *App) fetchGitHubReleases() ([]githubRelease, error) {
	apiURL := githubAPIBase + "/releases?per_page=100"
	urls := []string{apiURL, githubProxyBase + apiURL}
	var lastErr error
	for _, u := range urls {
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", updateUserAgent)
		req.Header.Set("Accept", "application/vnd.github+json")
		client := &http.Client{Timeout: 20 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			return nil, fmt.Errorf("rate_limited")
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("http status %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		var releases []githubRelease
		if err := json.Unmarshal(body, &releases); err != nil {
			lastErr = err
			continue
		}
		return releases, nil
	}
	return nil, lastErr
}

func resolveChannelRelease(releases []githubRelease, channel string) *githubRelease {
	keyword := validUpdateChannels[channel]
	for i := range releases {
		rel := &releases[i]
		if channel == "stable" {
			if !rel.Prerelease {
				return rel
			}
			continue
		}
		if rel.Prerelease && keyword != "" && strings.Contains(strings.ToLower(rel.TagName), keyword) {
			return rel
		}
	}
	return nil
}

func filterUpdateAssets(assets []githubAsset) []ReleaseAsset {
	result := []ReleaseAsset{}
	for _, asset := range assets {
		lower := strings.ToLower(asset.Name)
		var kind string
		switch {
		case strings.HasSuffix(lower, ".exe"):
			kind = "exe"
		case strings.HasSuffix(lower, ".7z"):
			kind = "7z"
		default:
			continue
		}
		result = append(result, ReleaseAsset{
			Name:        asset.Name,
			Size:        asset.Size,
			DownloadURL: asset.DownloadURL,
			Kind:        kind,
		})
	}
	return result
}

func classifyUpdateError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "rate_limited"):
		return "rate_limited"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline exceeded"), strings.Contains(msg, "context deadline"):
		return "network_timeout"
	case strings.Contains(msg, "connection refused"):
		return "connection_refused"
	case strings.Contains(msg, "no such host"), strings.Contains(msg, "dns"):
		return "dns_error"
	case strings.Contains(msg, "proxy"):
		return "proxy_error"
	default:
		return "api_error"
	}
}

func parseVersionParts(v string) ([]int, []string) {
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
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
	switch normalizeReleaseChannel(ch) {
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

func channelFromTag(tag string) string {
	lower := strings.ToLower(tag)
	switch {
	case strings.Contains(lower, "-alpha"):
		return "alpha"
	case strings.Contains(lower, "-beta"):
		return "beta"
	case strings.Contains(lower, "-rc"):
		return "rc"
	default:
		return "stable"
	}
}

func preNum(pre []string) int {
	for _, p := range pre {
		if n, err := strconv.Atoi(p); err == nil {
			return n
		}
	}
	return 0
}

func compareReleaseVersions(current, currentChannel, target, targetChannel string) int {
	cn, cp := parseVersionParts(current)
	tn, tp := parseVersionParts(target)
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
	cr := channelRank(currentChannel)
	tr := channelRank(targetChannel)
	if tr > cr {
		return -1
	}
	if tr < cr {
		return 1
	}
	a := preNum(cp)
	b := preNum(tp)
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func (a *App) DownloadUpdateAsset(assetURL string) (DownloadResult, error) {
	fileName := filepath.Base(strings.SplitN(assetURL, "?", 2)[0])
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = "snishaper-update.bin"
	}
	dir := filepath.Join(os.TempDir(), "snishaper-update")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return DownloadResult{}, err
	}
	dest := filepath.Join(dir, fileName)
	urls := []string{assetURL}
	if strings.HasPrefix(assetURL, "https://") {
		urls = append(urls, githubProxyBase+assetURL)
	}
	var lastErr error
	for _, u := range urls {
		if err := a.downloadFileWithProgress(u, dest, fileName); err != nil {
			lastErr = err
			a.appendLog("[update] Download attempt failed: " + u + " -> " + err.Error())
			continue
		}
		return DownloadResult{LocalPath: dest, Size: fileSize(dest)}, nil
	}
	return DownloadResult{}, lastErr
}

func (a *App) downloadFileWithProgress(url, dest, name string) error {
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	client := &http.Client{Transport: transport}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http status %d", resp.StatusCode)
	}
	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		if _, statErr := os.Stat(tmp); statErr == nil {
			os.Remove(tmp)
		}
	}()
	total := resp.ContentLength
	var received int64
	buf := make([]byte, 256*1024)
	lastEmit := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			received += int64(n)
			if time.Since(lastEmit) > 150*time.Millisecond {
				lastEmit = time.Now()
				a.emitDownloadProgress(name, received, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	a.emitDownloadProgress(name, received, total)
	return nil
}

func (a *App) emitDownloadProgress(name string, received, total int64) {
	percent := 0.0
	if total > 0 {
		percent = float64(received) / float64(total) * 100
	}
	application.InvokeAsync(func() {
		if a.mainWindow == nil || a.shouldQuit {
			return
		}
		a.mainWindow.EmitEvent("update:download_progress", map[string]interface{}{
			"asset_name": name,
			"received":   received,
			"total":      total,
			"percent":    percent,
		})
	})
}

func fileSize(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}

func (a *App) InstallUpdateAsset(localPath string) error {
	lower := strings.ToLower(localPath)
	switch {
	case strings.HasSuffix(lower, ".exe"):
		return a.launchInstaller(localPath)
	case strings.HasSuffix(lower, ".7z"):
		return a.installSevenZ(localPath)
	default:
		return fmt.Errorf("unsupported update file type: %s", localPath)
	}
}

func (a *App) launchInstaller(localPath string) error {
	a.appendLog("[update] Launching installer: " + localPath)
	cmd := exec.Command(localPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start installer: %v", err)
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func (a *App) installSevenZ(localPath string) error {
	sevenZ, err := findSevenZ()
	if err != nil {
		return fmt.Errorf("sevenzip_missing")
	}
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate application directory: %v", err)
	}
	execDir := filepath.Dir(execPath)
	if !isDirWritable(execDir) {
		return fmt.Errorf("dir_not_writable")
	}
	base := filepath.Join(os.TempDir(), "snishaper-update")
	stage := filepath.Join(base, "stage")
	os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	cmd := exec.Command(sevenZ, "x", localPath, "-o"+stage, "-y")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(stage)
		a.appendLog("[update] 7z extract failed: " + string(out))
		return fmt.Errorf("extract_failed")
	}
	if _, err := os.Stat(filepath.Join(stage, "snishaper.exe")); err != nil {
		os.RemoveAll(stage)
		return fmt.Errorf("bad_archive")
	}
	script := filepath.Join(base, "apply-update.cmd")
	content := buildUpdateScript(stage, execDir, base)
	if err := os.WriteFile(script, []byte(content), 0644); err != nil {
		return err
	}
	launcher := exec.Command("cmd", "/c", "\""+script+"\"")
	launcher.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := launcher.Start(); err != nil {
		return fmt.Errorf("failed to launch updater script: %v", err)
	}
	go func() {
		_ = launcher.Wait()
	}()
	a.appendLog("[update] 7z update script launched")
	return nil
}

func buildUpdateScript(stage, execDir, base string) string {
	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	sb.WriteString("chcp 65001 > nul\r\n")
	sb.WriteString("timeout /t 1 /nobreak > nul\r\n")
	sb.WriteString("taskkill /f /im snishaper.exe > nul 2>&1\r\n")
	sb.WriteString("timeout /t 3 /nobreak > nul\r\n")
	sb.WriteString("xcopy /y /e /q /i \"" + filepath.Join(stage, "*") + "\" \"" + execDir + "\" > nul 2>&1\r\n")
	sb.WriteString("if errorlevel 1 (\r\n")
	sb.WriteString("  timeout /t 3 /nobreak > nul\r\n")
	sb.WriteString("  xcopy /y /e /q /i \"" + filepath.Join(stage, "*") + "\" \"" + execDir + "\" > nul 2>&1\r\n")
	sb.WriteString(")\r\n")
	sb.WriteString("start \"\" \"" + filepath.Join(execDir, "snishaper.exe") + "\"\r\n")
	sb.WriteString("timeout /t 1 /nobreak > nul\r\n")
	sb.WriteString("rmdir /s /q \"" + base + "\"\r\n")
	return sb.String()
}

func findSevenZ() (string, error) {
	if p, err := exec.LookPath("7z"); err == nil {
		return p, nil
	}
	if p, err := findSevenZByRegistry(); err == nil {
		return p, nil
	}
	candidates := []string{
		`C:\Program Files\7-Zip\7z.exe`,
		`C:\Program Files (x86)\7-Zip\7z.exe`,
		filepath.Join(os.Getenv("ProgramFiles"), "7-Zip", "7z.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "7-Zip", "7z.exe"),
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("7-zip not found")
}

func findSevenZByRegistry() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\7-Zip`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()
	path, _, err := k.GetStringValue("Path")
	if err != nil {
		return "", err
	}
	exe := filepath.Join(path, "7z.exe")
	if _, err := os.Stat(exe); err != nil {
		return "", err
	}
	return exe, nil
}

func isDirWritable(dir string) bool {
	probe := filepath.Join(dir, ".update-probe")
	f, err := os.Create(probe)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(probe)
	return true
}
