package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/admpub/go-download/v2"
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
	"direct":               "",
	"down.mxw.qzz.io":      "https://down.mxw.qzz.io/",
	"gh-proxy.org":         "https://gh-proxy.org/",
	"v4.gh-proxy.org":      "https://v4.gh-proxy.org/",
	"v6.gh-proxy.org":      "https://v6.gh-proxy.org/",
	"cdn.gh-proxy.org":     "https://cdn.gh-proxy.org/",
	"axisnow.gh-proxy.org": "https://axisnow.gh-proxy.org/",
	"custom":               "",
}

const defaultDownloadSource = "down.mxw.qzz.io"

var buildVersion string
var buildChannel string

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
	if ch := strings.TrimSpace(buildChannel); ch != "" {
		return normalizeReleaseChannel(ch)
	}
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
	threshold := channelRank(channel)
	var best *githubRelease
	for i := range releases {
		rel := &releases[i]
		if releaseRank(*rel) < threshold {
			continue
		}
		if best == nil {
			best = rel
			continue
		}
		if compareReleaseVersions(
			strings.TrimPrefix(best.TagName, "v"), channelFromTag(best.TagName),
			strings.TrimPrefix(rel.TagName, "v"), channelFromTag(rel.TagName),
		) == -1 {
			best = rel
		}
	}
	return best
}

func releaseRank(rel githubRelease) int {
	if !rel.Prerelease {
		return 3
	}
	switch channelFromTag(rel.TagName) {
	case "rc":
		return 2
	case "beta":
		return 1
	default:
		return 0
	}
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

const (
	defaultDownloadConcurrency = 10
	defaultDownloadChunkSize   = 8 * 1024 * 1024
)

func (a *App) getDownloadConcurrency() int {
	if a.downloadConcurrency > 0 {
		return a.downloadConcurrency
	}
	n := defaultDownloadConcurrency
	if s := strings.TrimSpace(os.Getenv("DOWNLOAD_CONCURRENCY")); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			n = v
		}
	}
	a.downloadConcurrency = n
	return n
}

func (a *App) getDownloadChunkSize() int64 {
	if a.downloadChunkSize > 0 {
		return a.downloadChunkSize
	}
	n := int64(defaultDownloadChunkSize)
	if s := strings.TrimSpace(os.Getenv("DOWNLOAD_CHUNK_SIZE")); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v > 0 {
			n = v
		}
	}
	a.downloadChunkSize = n
	return n
}

func (a *App) downloadConcurrencyFn() download.ConcurrencyFn {
	conc := a.getDownloadConcurrency()
	chunk := a.getDownloadChunkSize()
	return func(size int64) int {
		n := conc
		if chunk > 0 && size > 0 {
			if byChunk := int(size / chunk); byChunk > 0 && byChunk < n {
				n = byChunk
			}
		}
		if n < 1 {
			n = 1
		}
		return n
	}
}

type downloadProgress struct {
	a         *App
	name      string
	mu        sync.Mutex
	received  int64
	total     int64
	lastEmit  time.Time
	lastBytes int64
}

type progressReader struct {
	prog *downloadProgress
	r    io.Reader
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	if n > 0 {
		p.prog.add(int64(n))
	}
	return n, err
}

func (p *downloadProgress) addTotal(n int64) {
	p.mu.Lock()
	p.total += n
	p.mu.Unlock()
}

func (p *downloadProgress) add(n int64) {
	p.mu.Lock()
	p.received += n
	now := time.Now()
	if now.Sub(p.lastEmit) <= 150*time.Millisecond {
		p.mu.Unlock()
		return
	}
	elapsed := now.Sub(p.lastEmit).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(p.received-p.lastBytes) / elapsed
	}
	p.lastEmit = now
	p.lastBytes = p.received
	received, total, name := p.received, p.total, p.name
	a := p.a
	p.mu.Unlock()
	a.emitDownloadProgress(name, received, total, speed)
}

func (p *downloadProgress) finish(written int64) {
	p.mu.Lock()
	received := p.received
	if written > received {
		received = written
	}
	total := p.total
	if total < received {
		total = received
	}
	name := p.name
	a := p.a
	p.mu.Unlock()
	a.emitDownloadProgress(name, received, total, 0)
}

func (a *App) downloadFileWithProgress(url, dest, name string) error {
	if a.ctx.Err() != nil {
		return a.ctx.Err()
	}
	transport := &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	client := &http.Client{Transport: transport}

	progress := &downloadProgress{
		a:        a,
		name:     name,
		lastEmit: time.Now(),
	}

	options := &download.Options{
		Concurrency: a.downloadConcurrencyFn(),
		Client: func() http.Client {
			return *client
		},
		Request: func(r *http.Request) {
			r.Header.Set("User-Agent", updateUserAgent)
		},
		Proxy: func(_ string, _ int, size int64, r io.Reader) io.Reader {
			progress.addTotal(size)
			return &progressReader{prog: progress, r: r}
		},
	}

	f, err := download.OpenContext(a.ctx, url, options)
	if err != nil {
		return err
	}
	defer f.Close()

	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() {
		out.Close()
		if _, statErr := os.Stat(tmp); statErr == nil {
			os.Remove(tmp)
		}
	}()

	written, err := io.Copy(out, f)
	if err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	progress.finish(written)
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
	err := a.installUpdateAsset(localPath)
	if err == nil {
		a.SetPendingUpdate("")
	}
	return err
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
