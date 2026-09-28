package app

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"snishaper/proxy"
)

const rulesRemoteURL = "https://raw.githubusercontent.com/" + githubRepo + "/main/rules/config.json"

const autoRulesUpdateDelay = 10 * time.Second

type RulesUpdateResult struct {
	UpToDate bool     `json:"up_to_date"`
	Updated  bool     `json:"updated"`
	Added    int      `json:"added"`
	Changed  int      `json:"changed"`
	Kept     int      `json:"kept"`
	Source   string   `json:"source,omitempty"`
	Error    string   `json:"error,omitempty"`
	Details  []string `json:"details,omitempty"`
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

	sum256 := sha256.Sum256(body)
	hash := hex.EncodeToString(sum256[:])

	if hash == a.ruleManager.RemoteRulesHash() {
		a.appendLog("[rules-update] remote rules already applied (" + hash[:12] + ")")
		return RulesUpdateResult{UpToDate: true, Source: source}
	}

	var remote proxy.RulesConfig
	if err := json.Unmarshal(body, &remote); err != nil {
		msg := "invalid remote rules file: " + err.Error()
		a.appendLog("[rules-update] " + msg)
		return RulesUpdateResult{Error: msg, Source: source}
	}

	summary, err := a.ruleManager.MergeRemoteRules(remote, hash)
	if err != nil {
		a.appendLog("[rules-update] merge failed: " + err.Error())
		return RulesUpdateResult{Error: err.Error(), Source: source}
	}

	if !summary.Changed() {
		a.appendLog("[rules-update] remote file differs but all rules are current")
		return RulesUpdateResult{UpToDate: true, Source: source}
	}

	res := RulesUpdateResult{
		Updated: true,
		Added:   summary.AddedGroups + summary.AddedUpstreams + summary.AddedDNSNodes + summary.AddedECHProfiles + summary.AddedNAT64,
		Changed: summary.UpdatedGroups + summary.UpdatedUpstreams,
		Kept:    summary.KeptGroups + summary.KeptUpstreams,
		Source:  source,
		Details: summary.Details,
	}
	a.appendLog(fmt.Sprintf("[rules-update] applied from %s: %d added, %d updated, %d kept local", source, res.Added, res.Changed, res.Kept))
	return res
}

func (a *App) rulesFetchCandidates() []rulesFetchCandidate {
	direct := &http.Client{
		Timeout: 12 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	prefixes := []struct{ label, prefix string }{}
	seen := map[string]bool{}
	add := func(label, prefix string) {
		if seen[label] {
			return
		}
		seen[label] = true
		prefixes = append(prefixes, struct{ label, prefix string }{label, prefix})
	}
	switch src := a.GetDownloadSource(); src {
	case "custom":
		add("custom", a.GetCustomDownloadSource())
	case "direct", "":
	default:
		if p, ok := downloadSources[src]; ok {
			add(src, p)
		}
	}
	add("direct", "")
	for _, name := range downloadSourceOrder {
		add(name, downloadSources[name])
	}
	add("gh.llkk.cc", githubProxyBase)

	candidates := make([]rulesFetchCandidate, 0, len(prefixes)*2)
	for _, p := range prefixes {
		candidates = append(candidates, rulesFetchCandidate{label: p.label, url: p.prefix + rulesRemoteURL, client: direct})
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
			for _, p := range prefixes {
				candidates = append(candidates, rulesFetchCandidate{label: p.label + "+proxy", url: p.prefix + rulesRemoteURL, client: viaProxy})
			}
		}
	}

	return candidates
}

func (a *App) fetchRemoteRules() ([]byte, string, error) {
	candidates := a.rulesFetchCandidates()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type outcome struct {
		body   []byte
		source string
		err    error
	}

	results := make(chan outcome, len(candidates))
	var wg sync.WaitGroup
	for _, c := range candidates {
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
		lastErr = fmt.Errorf("no download source reachable")
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
