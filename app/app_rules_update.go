package app

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"snishaper/proxy"
)

const rulesRemoteURL = "https://raw.githubusercontent.com/" + githubRepo + "/main/rules/config.json"

const autoRulesUpdateDelay = 10 * time.Second

type RulesUpdateResult struct {
	UpToDate        bool     `json:"up_to_date"`
	Updated         bool     `json:"updated"`
	Added           int      `json:"added"`
	Changed         int      `json:"changed"`
	Kept            int      `json:"kept"`
	Source          string   `json:"source,omitempty"`
	LocalHash       string   `json:"local_hash,omitempty"`
	RemoteHash      string   `json:"remote_hash,omitempty"`
	RemoteUpdatedAt string   `json:"remote_updated_at,omitempty"`
	Error           string   `json:"error,omitempty"`
	Details         []string `json:"details,omitempty"`
}

type rulesFetchCandidate struct {
	label  string
	url    string
	client *http.Client
}

func (a *App) GetAutoUpdateRules() bool {
	return a.ruleManager.GetAutoUpdateRules()
}

func (a *App) SetAutoUpdateRules(enabled bool) error {
	a.appendLog(fmt.Sprintf("[action] SetAutoUpdateRules called: %v", enabled))
	return a.ruleManager.SetAutoUpdateRules(enabled)
}

func (a *App) UpdateRules() RulesUpdateResult {
	a.appendLog("[rules-update] manual update requested")
	return a.syncRemoteRules()
}

func (a *App) syncRemoteRules() RulesUpdateResult {
	body, source, err := a.fetchRemoteRules()
	if err != nil {
		a.appendLog("[rules-update] fetch failed: " + err.Error())
		return RulesUpdateResult{Error: err.Error()}
	}

	// 校验下载内容与官方 GitHub API 报告的 git blob SHA 一致，
	// 防止被篡改的镜像注入规则。SNISHAPER_ALLOW_UNVERIFIED_RULES=1
	// 是给离线环境留的显式逃生舱。
	if rulesIntegrityBypassEnabled() {
		a.appendLog("[rules-update] WARNING: integrity verification bypassed via SNISHAPER_ALLOW_UNVERIFIED_RULES")
	} else {
		expectedSHA, shaErr := a.fetchOfficialRulesBlobSHA()
		if shaErr != nil {
			msg := "rules integrity check unavailable (official GitHub API): " + shaErr.Error()
			a.appendLog("[rules-update] " + msg)
			return RulesUpdateResult{Error: msg, Source: source}
		}
		if got := rulesBlobSHA(body); !strings.EqualFold(got, expectedSHA) {
			msg := fmt.Sprintf("rules integrity check failed: expected blob sha %s, got %s", expectedSHA, got)
			a.appendLog("[rules-update] " + msg)
			return RulesUpdateResult{Error: msg, Source: source}
		}
	}

	sum256 := sha256.Sum256(body)
	hash := hex.EncodeToString(sum256[:])

	remoteUpdatedAt := a.fetchRemoteRulesUpdatedAt()

	if hash == a.ruleManager.RemoteRulesHash() {
		a.appendLog("[rules-update] remote rules already applied (" + hash[:12] + ")")
		return RulesUpdateResult{
			UpToDate:        true,
			Source:          source,
			LocalHash:       a.ruleManager.CurrentRulesHash(),
			RemoteHash:      hash,
			RemoteUpdatedAt: remoteUpdatedAt,
		}
	}

	var remote proxy.RulesConfig
	if err := json.Unmarshal(body, &remote); err != nil {
		msg := "invalid remote rules file: " + err.Error()
		a.appendLog("[rules-update] " + msg)
		return RulesUpdateResult{Error: msg, Source: source}
	}

	localHash := a.ruleManager.CurrentRulesHash()
	summary, err := a.ruleManager.MergeRemoteRules(remote, hash)
	if err != nil {
		a.appendLog("[rules-update] merge failed: " + err.Error())
		return RulesUpdateResult{Error: err.Error(), Source: source}
	}

	if !summary.Changed() {
		a.appendLog("[rules-update] remote file differs but all rules are current")
		return RulesUpdateResult{
			UpToDate:        true,
			Source:          source,
			LocalHash:       a.ruleManager.CurrentRulesHash(),
			RemoteHash:      hash,
			RemoteUpdatedAt: remoteUpdatedAt,
		}
	}

	res := RulesUpdateResult{
		Updated:         true,
		Added:           summary.AddedGroups + summary.AddedUpstreams + summary.AddedDNSNodes + summary.AddedECHProfiles + summary.AddedNAT64,
		Changed:         summary.UpdatedGroups + summary.UpdatedUpstreams,
		Kept:            summary.KeptGroups + summary.KeptUpstreams,
		Source:          source,
		LocalHash:       localHash,
		RemoteHash:      hash,
		RemoteUpdatedAt: remoteUpdatedAt,
		Details:         summary.Details,
	}
	a.appendLog(fmt.Sprintf("[rules-update] applied from %s: %d added, %d updated, %d kept local",
		source, res.Added, res.Changed, res.Kept))
	return res
}

// rulesIntegrityBypassEnabled reports whether the user explicitly opted out of
// rules integrity verification.
func rulesIntegrityBypassEnabled() bool {
	return os.Getenv("SNISHAPER_ALLOW_UNVERIFIED_RULES") == "1"
}

// rulesBlobSHA computes the git blob hash of the rules file:
// sha1("blob <size>\x00" + content). It matches the "sha" field GitHub's
// contents API reports for the file.
func rulesBlobSHA(body []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(body))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// fetchOfficialRulesBlobSHA returns the git blob SHA of the upstream rules
// file from the official GitHub contents API. Mirror responses are never
// used here.
func (a *App) fetchOfficialRulesBlobSHA() (string, error) {
	apiURL := githubAPIBase + "/contents/rules/config.json"
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", updateUserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	sha := strings.ToLower(strings.TrimSpace(payload.SHA))
	if sha == "" {
		return "", fmt.Errorf("official GitHub API did not report a blob sha")
	}
	return sha, nil
}

// fetchRemoteRulesUpdatedAt reports when the remote rules file last changed,
// read from the GitHub commits API. It is best effort: an empty result only
// means the caller should fall back to the fetch time.
func (a *App) fetchRemoteRulesUpdatedAt() string {
	apiURL := "https://api.github.com/repos/" + githubRepo + "/commits?path=rules/config.json&per_page=1"
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	var payload []struct {
		Commit struct {
			Committer struct {
				Date string `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	for _, e := range a.downloadSourceEntries(sourcePurposeRaw) {
		req, err := http.NewRequest(http.MethodGet, e.prefix+apiURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", updateUserAgent)
		req.Header.Set("Accept", "application/vnd.github+json")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		if err := json.Unmarshal(body, &payload); err != nil || len(payload) == 0 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, payload[0].Commit.Committer.Date)
		if err != nil {
			continue
		}
		return ts.Local().Format("2006-01-02 15:04")
	}
	return ""
}

func (a *App) rulesFetchCandidates() []rulesFetchCandidate {
	direct := &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	entries := a.downloadSourceEntries(sourcePurposeRaw)
	candidates := make([]rulesFetchCandidate, 0, len(entries)*2)
	for _, e := range entries {
		candidates = append(candidates, rulesFetchCandidate{label: e.label, url: e.prefix + rulesRemoteURL, client: direct})
	}

	if a.IsProxyRunning() {
		if proxyURL, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", a.GetListenPort())); err == nil {
			viaProxy := &http.Client{
				Timeout: 12 * time.Second,
				Transport: &http.Transport{
					Proxy:           http.ProxyURL(proxyURL),
					TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
				},
			}
			for _, e := range entries {
				candidates = append(candidates, rulesFetchCandidate{label: e.label + "+proxy", url: e.prefix + rulesRemoteURL, client: viaProxy})
			}
		}
	}

	return candidates
}

// fetchRemoteRules tries the configured download source first and only then
// races the remaining mirrors, so the choice made in Settings is honoured.
func (a *App) fetchRemoteRules() ([]byte, string, error) {
	candidates := a.rulesFetchCandidates()
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no download source available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if body, err := fetchRulesFile(ctx, candidates[0].client, candidates[0].url); err == nil {
		return body, candidates[0].label, nil
	} else {
		a.appendLog(fmt.Sprintf("[rules-update] %s failed: %v", candidates[0].label, err))
	}

	rest := candidates[1:]
	if len(rest) == 0 {
		return nil, "", fmt.Errorf("no reachable download source")
	}

	type outcome struct {
		body   []byte
		source string
		err    error
	}

	results := make(chan outcome, len(rest))
	var wg sync.WaitGroup
	for _, c := range rest {
		wg.Add(1)
		go func(c rulesFetchCandidate) {
			defer wg.Done()
			body, err := fetchRulesFile(ctx, c.client, c.url)
			select {
			case <-ctx.Done():
			case results <- outcome{body: body, source: c.label, err: err}:
			}
		}(c)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var lastErr error
	for r := range results {
		if r.err == nil {
			cancel()
			return r.body, r.source, nil
		}
		lastErr = r.err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no reachable download source")
	}
	return nil, "", lastErr
}

func fetchRulesFile(ctx context.Context, client *http.Client, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", updateUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, err
	}

	var probe struct {
		SiteGroups []json.RawMessage `json:"site_groups"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return nil, fmt.Errorf("not a rules file: %v", err)
	}
	if len(probe.SiteGroups) == 0 {
		return nil, fmt.Errorf("rules file carries no site groups")
	}
	return body, nil
}
