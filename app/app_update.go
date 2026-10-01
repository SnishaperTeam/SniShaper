package app

import (
	"crypto/sha256"
	"encoding/hex"
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
)

const (
	githubRepo      = "SnishaperTeam/SniShaper"
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
	"smart":                "",
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

const smartSourceCacheTTL = 10 * time.Minute

const (
	sourcePurposeRelease = "release"
	sourcePurposeRaw     = "raw"
)

var sourceProbes = map[string]string{
	sourcePurposeRelease: "https://github.com/" + githubRepo + "/releases/latest",
	sourcePurposeRaw:     "https://raw.githubusercontent.com/" + githubRepo + "/main/rules/config.json",
}

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
	// Digest is the "sha256:..." digest reported by the GitHub Releases API.
	// Only the value from the official api.github.com endpoint is trusted.
	Digest string `json:"digest"`
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

// UpdateChannels lists the channels CheckUpdate accepts, so callers can show
// the valid values instead of hardcoding them.
func UpdateChannels() []string {
	names := make([]string, 0, len(validUpdateChannels))
	for name := range validUpdateChannels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DownloadSources lists the download source prefixes the updater understands.
func DownloadSources() []string {
	names := make([]string, 0, len(downloadSources))
	for name := range downloadSources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
			results[i] = measureSourceLatency(tg.name, tg.prefix, sourceProbes[sourcePurposeRelease])
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

func measureSourceLatency(name, prefix, probe string) DownloadSourceStatus {
	target := probe
	if prefix != "" {
		target = prefix + probe
	}
	st := DownloadSourceStatus{Name: name, URL: target}
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	req, err := http.NewRequest(http.MethodHead, target, nil)
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
	if resp.StatusCode >= 400 {
		st.Error = fmt.Sprintf("http status %d", resp.StatusCode)
		return st
	}
	st.OK = true
	return st
}

type downloadSourceEntry struct {
	label  string
	prefix string
}

// rankedSourceOrder measures every mirror against the probe for the given
// purpose and returns the reachable ones fastest first, so the "smart" source
// always points at the quickest mirror for that kind of request.
func (a *App) rankedSourceOrder(purpose string) []string {
	probe, ok := sourceProbes[purpose]
	if !ok {
		probe = sourceProbes[sourcePurposeRelease]
	}

	a.sourceRankMu.Lock()
	cached, at := a.sourceRankCache[purpose], a.sourceRankAt[purpose]
	a.sourceRankMu.Unlock()
	if len(cached) > 0 && time.Since(at) < smartSourceCacheTTL {
		return cached
	}

	names := append([]string{"direct"}, downloadSourceOrder...)
	type sample struct {
		name      string
		latencyMS int64
		ok        bool
	}
	samples := make([]sample, len(names))
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			st := measureSourceLatency(name, downloadSources[name], probe)
			samples[i] = sample{name: name, latencyMS: st.LatencyMS, ok: st.OK}
		}(i, name)
	}
	wg.Wait()

	sort.SliceStable(samples, func(i, j int) bool {
		if samples[i].ok != samples[j].ok {
			return samples[i].ok
		}
		return samples[i].latencyMS < samples[j].latencyMS
	})
	order := make([]string, 0, len(samples))
	for _, s := range samples {
		if s.ok {
			order = append(order, s.name)
		}
	}
	if len(order) == 0 {
		order = names
	}

	a.sourceRankMu.Lock()
	if a.sourceRankCache == nil {
		a.sourceRankCache = map[string][]string{}
		a.sourceRankAt = map[string]time.Time{}
	}
	a.sourceRankCache[purpose], a.sourceRankAt[purpose] = order, time.Now()
	a.sourceRankMu.Unlock()
	a.appendLog(fmt.Sprintf("[update] smart source ranking (%s): %s", purpose, strings.Join(order, " > ")))
	return order
}

// downloadSourceEntries lists the mirrors to try for the configured download
// source, in order: the configured one first, then direct, then the rest.
func (a *App) downloadSourceEntries(purpose string) []downloadSourceEntry {
	src := a.GetDownloadSource()
	entries := []downloadSourceEntry{}
	seen := map[string]bool{}
	add := func(label, prefix string) {
		if seen[prefix] {
			return
		}
		seen[prefix] = true
		entries = append(entries, downloadSourceEntry{label: label, prefix: prefix})
	}

	switch src {
	case "custom":
		if p := strings.TrimRight(a.GetCustomDownloadSource(), "/"); p != "" {
			add("custom", p+"/")
		}
	case "smart":
		for _, name := range a.rankedSourceOrder(purpose) {
			add(name, downloadSources[name])
		}
	case "direct", "":
		add("direct", "")
	default:
		if p, ok := downloadSources[src]; ok {
			add(src, p)
		}
	}

	add("direct", "")
	for _, name := range downloadSourceOrder {
		if name == src {
			continue
		}
		add(name, downloadSources[name])
	}
	return entries
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
	// Official API first; the mirror is only a fallback for listing releases.
	releases, err := fetchReleasesFromURL(apiURL)
	if err == nil {
		return releases, nil
	}
	if mirrorReleases, mirrorErr := fetchReleasesFromURL(githubProxyBase + apiURL); mirrorErr == nil {
		return mirrorReleases, nil
	}
	return nil, err
}

func fetchReleasesFromURL(apiURL string) ([]githubRelease, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", updateUserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		resp.Body.Close()
		return nil, fmt.Errorf("rate_limited")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, err
	}
	return releases, nil
}

// fetchOfficialAssetDigest resolves the expected SHA-256 of a release asset
// from the official GitHub API. Mirror responses are never used here.
func (a *App) fetchOfficialAssetDigest(assetURL string) (string, error) {
	releases, err := fetchReleasesFromURL(githubAPIBase + "/releases?per_page=100")
	if err != nil {
		return "", err
	}
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if asset.DownloadURL != assetURL {
				continue
			}
			digest := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(asset.Digest)), "sha256:")
			if digest == "" {
				return "", fmt.Errorf("official GitHub API did not provide a digest for %s", asset.Name)
			}
			return digest, nil
		}
	}
	return "", fmt.Errorf("asset not found on official GitHub releases: %s", assetURL)
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
		case strings.HasSuffix(lower, ".7z") && !strings.Contains(lower, "_x64") && !strings.Contains(lower, "_x86") && !strings.Contains(lower, "_arm64") && !strings.Contains(lower, "unsigned"):
			kind = "7z"
		case strings.HasSuffix(lower, ".tar.gz"):
			// Portable bundle used by the Linux build, and the only archive
			// the Linux installer can unpack on its own.
			kind = "tar.gz"
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

func buildDownloadURLs(assetURL string, entries []downloadSourceEntry) []string {
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
	for _, e := range entries {
		add(e.prefix + assetURL)
	}
	add(assetURL)
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

// fileSHA256Hex streams a file through SHA-256 and returns the lowercase hex digest.
func fileSHA256Hex(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (a *App) DownloadUpdateAsset(assetURL string) (DownloadResult, error) {
	// Resolve the expected SHA-256 from the official GitHub API before
	// downloading. Without a trusted digest the update is rejected outright.
	expectedSHA, err := a.fetchOfficialAssetDigest(assetURL)
	if err != nil {
		a.appendLog("[update] Integrity check unavailable (official GitHub API): " + err.Error())
		return DownloadResult{}, fmt.Errorf("update integrity check unavailable: %v", err)
	}

	fileName := filepath.Base(strings.SplitN(assetURL, "?", 2)[0])
	if fileName == "." || fileName == "/" || fileName == "" {
		fileName = "snishaper-update.bin"
	}
	dir := filepath.Join(os.TempDir(), "snishaper-update")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return DownloadResult{}, err
	}
	dest := filepath.Join(dir, fileName)
	urls := buildDownloadURLs(assetURL, a.downloadSourceEntries(sourcePurposeRelease))
	var lastErr error
	for _, u := range urls {
		if err := a.downloadFileWithProgress(u, dest, fileName); err != nil {
			lastErr = err
			a.appendLog("[update] Download attempt failed: " + u + " -> " + err.Error())
			continue
		}
		// Verify the downloaded bytes; a tampered mirror is treated like a
		// failed download and the next source is tried.
		sum, hashErr := fileSHA256Hex(dest)
		if hashErr != nil {
			lastErr = hashErr
			os.Remove(dest)
			a.appendLog("[update] Hashing failed for " + dest + ": " + hashErr.Error())
			continue
		}
		if !strings.EqualFold(sum, expectedSHA) {
			lastErr = fmt.Errorf("integrity check failed for %s: expected sha256 %s, got %s", fileName, expectedSHA, sum)
			os.Remove(dest)
			a.appendLog("[update] " + lastErr.Error())
			continue
		}
		a.setPendingUpdateVerified(dest, expectedSHA)
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
	a.invokeAsync(func() {
		if !a.hasUI() || a.shouldQuit {
			return
		}
		a.emitEvent("update:download_progress", map[string]interface{}{
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
	a.pendingUpdateSHA = ""
}

// setPendingUpdateVerified records the verified download together with the
// SHA-256 it was validated against, so the installer can re-check the file
// right before executing it.
func (a *App) setPendingUpdateVerified(path, expectedSHA string) {
	a.pendingUpdateMu.Lock()
	defer a.pendingUpdateMu.Unlock()
	a.pendingUpdatePath = path
	a.pendingUpdateSHA = expectedSHA
}

func (a *App) InstallUpdateAsset(localPath string) error {
	// Re-verify the pending download before executing it, so a file swapped
	// or modified on disk after the download cannot be installed.
	a.pendingUpdateMu.Lock()
	pendingPath, expectedSHA := a.pendingUpdatePath, a.pendingUpdateSHA
	a.pendingUpdateMu.Unlock()
	if expectedSHA != "" && strings.EqualFold(localPath, pendingPath) {
		sum, err := fileSHA256Hex(localPath)
		if err != nil {
			return fmt.Errorf("update integrity re-check failed: %v", err)
		}
		if !strings.EqualFold(sum, expectedSHA) {
			return fmt.Errorf("update file changed after download (sha256 mismatch), refusing to install")
		}
	}
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
