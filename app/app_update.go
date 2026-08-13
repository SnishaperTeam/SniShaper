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
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	githubRepo      = "SniShaper/SniShaper"
	githubAPIBase   = "https://api.github.com/repos/" + githubRepo
	githubProxyBase = "https://gh.llkk.cc/"
	updateUserAgent = "SniShaper-Update/1.0"
	relNS           = "http://schemas.snishaper.dev/release"
)

var downloadSourceOrder = []string{
	"down.mxw.qzz.io",
	"gh-proxy.org",
	"v4.gh-proxy.org",
	"v6.gh-proxy.org",
	"cdn.gh-proxy.org",
	"axisnow.gh-proxy.org",
}

var downloadSources = map[string]string{
	"direct":              "",
	"down.mxw.qzz.io":     "https://down.mxw.qzz.io/",
	"gh-proxy.org":        "https://gh-proxy.org/",
	"v4.gh-proxy.org":     "https://v4.gh-proxy.org/",
	"v6.gh-proxy.org":     "https://v6.gh-proxy.org/",
	"cdn.gh-proxy.org":    "https://cdn.gh-proxy.org/",
	"axisnow.gh-proxy.org": "https://axisnow.gh-proxy.org/",
	"custom":              "",
}

const defaultDownloadSource = "down.mxw.qzz.io"

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

func (a *App) GetDownloadSource() string {
	src := a.ruleManager.GetDownloadSource()
	if _, ok := downloadSources[src]; !ok {
		return defaultDownloadSource
	}
	return src
}

func (a *App) SetDownloadSource(src string) error {
	src = strings.ToLower(strings.TrimSpace(src))
	if _, ok := downloadSources[src]; !ok {
		return fmt.Errorf("invalid download source: %s", src)
	}
	a.appendLog("[update] Download source set to: " + src)
	return a.ruleManager.SetDownloadSource(src)
}

func (a *App) GetCustomDownloadSource() string {
	return a.ruleManager.GetCustomDownloadSource()
}

func (a *App) SetCustomDownloadSource(prefix string) error {
	prefix = strings.TrimSpace(prefix)
	a.appendLog("[update] Custom download source set to: " + prefix)
	return a.ruleManager.SetCustomDownloadSource(prefix)
}

type DownloadSourceStatus struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	LatencyMS int64  `json:"latency_ms"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

func (a *App) MeasureDownloadSources() []DownloadSourceStatus {
	type target struct{ name, prefix string }
	targets := []target{{name: "direct", prefix: ""}}
	for _, name := range downloadSourceOrder {
		targets = append(targets, target{name: name, prefix: downloadSources[name]})
	}
	results := make([]DownloadSourceStatus, len(targets))
	var wg sync.WaitGroup
	for i, tg := range targets {
		wg.Add(1)
		go func(i int, tg target) {
			defer wg.Done()
			results[i] = measureSourceLatency(tg.name, tg.prefix)
		}(i, tg)
	}
	wg.Wait()
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].OK != results[j].OK {
			return results[i].OK
		}
		return results[i].LatencyMS < results[j].LatencyMS
	})
	return results
}

func measureSourceLatency(name, prefix string) DownloadSourceStatus {
	probe := "https://github.com/SniShaper/SniShaper/releases/latest"
	if prefix != "" {
		probe = prefix + probe
	}
	st := DownloadSourceStatus{Name: name, URL: probe}
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	req, err := http.NewRequest(http.MethodHead, probe, nil)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	req.Header.Set("User-Agent", updateUserAgent)
	resp, err := client.Do(req)
	st.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		st.Error = err.Error()
		return st
	}
	resp.Body.Close()
	st.OK = true
	return st
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
		case strings.HasSuffix(lower, ".7z") && !strings.Contains(lower, "_x64.7z") && !strings.Contains(lower, "_x86.7z") && !strings.Contains(lower, "_arm64.7z") && !strings.Contains(lower, "unsigned"):
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

func buildDownloadURLs(assetURL, preferred, customPrefix string) []string {
	seen := map[string]bool{}
	var urls []string
	add := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	if !strings.HasPrefix(assetURL, "https://") {
		add(assetURL)
		return urls
	}
	if p := downloadSources[preferred]; p != "" {
		add(p + assetURL)
	} else if preferred == "custom" {
		add(strings.TrimRight(customPrefix, "/") + "/" + assetURL)
	}
	add(assetURL)
	for _, k := range downloadSourceOrder {
		if k == preferred {
			continue
		}
		if p := downloadSources[k]; p != "" {
			add(p + assetURL)
		}
	}
	return urls
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
	urls := buildDownloadURLs(assetURL, a.ruleManager.GetDownloadSource(), a.ruleManager.GetCustomDownloadSource())
	var lastErr error
	for _, u := range urls {
		if err := a.downloadFileWithProgress(u, dest, fileName); err != nil {
			lastErr = err
			a.appendLog("[update] Download attempt failed: " + u + " -> " + err.Error())
			continue
		}
		a.SetPendingUpdate(dest)
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
	var speed float64
	lastEmit := time.Now()
	lastBytes := int64(0)
	buf := make([]byte, 256*1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			received += int64(n)
			if time.Since(lastEmit) > 150*time.Millisecond {
				now := time.Now()
				if elapsed := now.Sub(lastEmit).Seconds(); elapsed > 0 {
					speed = float64(received-lastBytes) / elapsed
				}
				lastEmit = now
				lastBytes = received
				a.emitDownloadProgress(name, received, total, speed)
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
	a.emitDownloadProgress(name, received, total, speed)
	return nil
}

func (a *App) emitDownloadProgress(name string, received, total int64, speed float64) {
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
			"speed":      speed,
		})
	})
}

func fileSize(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}

func (a *App) GetPendingUpdate() string {
	a.pendingUpdateMu.Lock()
	defer a.pendingUpdateMu.Unlock()
	return a.pendingUpdatePath
}

func (a *App) SetPendingUpdate(path string) {
	a.pendingUpdateMu.Lock()
	defer a.pendingUpdateMu.Unlock()
	a.pendingUpdatePath = path
}

func (a *App) InstallUpdateAsset(localPath string) error {
	lower := strings.ToLower(localPath)
	var err error
	switch {
	case strings.HasSuffix(lower, ".exe"):
		err = a.launchInstaller(localPath)
	case strings.HasSuffix(lower, ".7z"):
		err = a.installSevenZ(localPath)
	default:
		err = fmt.Errorf("unsupported update file type: %s", localPath)
	}
	if err == nil {
		a.SetPendingUpdate("")
	}
	return err
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
	script := filepath.Join(base, "apply-update.ps1")
	content := buildUpdateScript(localPath, stage, execDir, base)
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)
	if err := os.WriteFile(script, bom, 0644); err != nil {
		return err
	}
	// 用 cmd /c start 启动：让 pwsh 拥有独立的新控制台窗口
	// （直接 exec 启动会继承 GUI 父进程的无效 std 句柄，新窗口收不到输出）
	launcher := exec.Command("cmd", "/c", "start", "SniShaper 更新", "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	launcher.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := launcher.Start(); err != nil {
		return fmt.Errorf("failed to launch updater script: %v", err)
	}
	go func() {
		_ = launcher.Wait()
	}()
	a.appendLog("[update] 7z update script launched (pwsh, visible window)")
	return nil
}

func buildUpdateScript(archive, stage, execDir, base string) string {
	return `$ErrorActionPreference = 'Continue'
$Host.UI.RawUI.WindowTitle = 'SniShaper 便携版更新'
Write-Host ''
Write-Host '========================================' -ForegroundColor Cyan
Write-Host '          SniShaper 便携版更新' -ForegroundColor Cyan
Write-Host '========================================' -ForegroundColor Cyan
Write-Host ''

$archive = '` + archive + `'
$stage = '` + stage + `'
$execDir = '` + execDir + `'
$base = '` + base + `'
$exe = Join-Path $execDir 'snishaper.exe'

Write-Host '[1/5] 正在解压更新包...' -ForegroundColor Yellow
& tar -xf $archive -C $stage | Out-Null
if ($LASTEXITCODE -ne 0 -or -not (Test-Path (Join-Path $stage 'snishaper.exe'))) {
    Write-Host '[错误] 解压更新包失败。' -ForegroundColor Red
    Read-Host '按回车键退出'
    exit 1
}
Write-Host '      解压完成。' -ForegroundColor Green

Write-Host '[2/5] 正在停止旧进程...' -ForegroundColor Yellow
Stop-Process -Name 'snishaper' -Force -ErrorAction SilentlyContinue
for ($i = 0; $i -lt 30; $i++) {
    if (-not (Get-Process -Name 'snishaper' -ErrorAction SilentlyContinue)) { break }
    Start-Sleep -Milliseconds 500
}
Write-Host '      旧进程已退出。' -ForegroundColor Green

Write-Host '[3/5] 正在覆盖程序文件...' -ForegroundColor Yellow
$ok = $false
for ($i = 0; $i -lt 10; $i++) {
    try {
        Copy-Item -Path (Join-Path $stage 'snishaper.exe') -Destination $exe -Force -ErrorAction Stop
        $ok = $true
        break
    } catch { Start-Sleep -Seconds 1 }
}
if (-not $ok) {
    Write-Host '[错误] 覆盖主程序失败，文件可能仍被占用。' -ForegroundColor Red
    Read-Host '按回车键退出'
    exit 2
}
Copy-Item -Path (Join-Path $stage 'rules') -Destination $execDir -Recurse -Force
Write-Host '      文件已更新（保留个人设置）。' -ForegroundColor Green

Write-Host '[4/5] 正在启动新版本...' -ForegroundColor Yellow
for ($i = 0; $i -lt 30; $i++) {
    if (Get-Process -Name 'snishaper' -ErrorAction SilentlyContinue) { break }
    Start-Process -FilePath $exe
    Start-Sleep -Seconds 3
}

Write-Host '[5/5] 清理临时文件...' -ForegroundColor Yellow
Start-Sleep -Seconds 1
Remove-Item -Path $base -Recurse -Force -ErrorAction SilentlyContinue

if (Get-Process -Name 'snishaper' -ErrorAction SilentlyContinue) {
    Write-Host ''
    Write-Host '更新完成，SniShaper 已重新启动。' -ForegroundColor Green
    Start-Sleep -Seconds 2
} else {
    Write-Host ''
    Write-Host '[警告] 未检测到程序进程，请手动打开 SniShaper。' -ForegroundColor Yellow
    Read-Host '按回车键退出'
    exit 3
}
`
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
