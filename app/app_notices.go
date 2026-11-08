package app

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	noticesEndpoint       = "https://jetcpp.ccwu.cc/api/notices?all=1"
	noticesMirrorEndpoint = "https://cdn.jsdelivr.net/gh/SnishaperTeam/Website@main/build/vue-app/notices/notices.json"
	noticesCacheTTL       = 10 * time.Minute
	noticesTimeout        = 8 * time.Second
	noticesCacheName      = "notices_cache.json"
)

type Notice struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	TitleEN     string `json:"title_en,omitempty"`
	TitleRU     string `json:"title_ru,omitempty"`
	Body        string `json:"body"`
	BodyEN      string `json:"body_en,omitempty"`
	BodyRU      string `json:"body_ru,omitempty"`
	Level       string `json:"level"`
	PublishedAt string `json:"published_at"`
	ExpiresAt   string `json:"expires_at,omitempty"`
	Link        string `json:"link,omitempty"`
	LinkText    string `json:"link_text,omitempty"`
	Pinned      bool   `json:"pinned"`
}

type NoticesPayload struct {
	Version   int      `json:"version"`
	UpdatedAt string   `json:"updated_at"`
	Notices   []Notice `json:"notices"`
	FetchedAt int64    `json:"fetched_at"`
	Source    string   `json:"source"`
	Cached    bool     `json:"cached"`
	Stale     bool     `json:"stale"`
	Error     string   `json:"error,omitempty"`
}

type noticeSource struct {
	name  string
	url   string
	proxy string
}

func (a *App) GetNotices(force bool) (*NoticesPayload, error) {
	a.noticesMu.Lock()
	if !force && a.noticesPayload != nil && time.Since(a.noticesFetched) < noticesCacheTTL {
		cached := *a.noticesPayload
		cached.Cached = true
		cached.Stale = false
		cached.Error = ""
		a.noticesMu.Unlock()
		return &cached, nil
	}
	a.noticesMu.Unlock()

	payload, err := a.fetchNotices()
	if err != nil {
		fallback := a.cachedNoticesSnapshot()
		if fallback == nil {
			return nil, err
		}
		out := *fallback
		out.Cached = true
		out.Stale = true
		out.Error = err.Error()
		a.noticesMu.Lock()
		a.noticesPayload = fallback
		a.noticesFetched = time.Now().Add(-noticesCacheTTL / 2)
		a.noticesMu.Unlock()
		return &out, nil
	}

	a.noticesMu.Lock()
	a.noticesPayload = payload
	a.noticesFetched = time.Now()
	a.noticesMu.Unlock()
	a.saveNoticesCache(payload)
	return payload, nil
}

func (a *App) cachedNoticesSnapshot() *NoticesPayload {
	if a.noticesPayload != nil {
		return a.noticesPayload
	}
	return a.loadNoticesCache()
}

func (a *App) localProxyURL() string {
	if a.core == nil && a.proxyServer == nil {
		return ""
	}
	running := false
	if a.core != nil {
		running = a.core.IsProxyRunning()
	} else if a.proxyServer != nil {
		running = a.proxyServer.IsRunning()
	}
	if !running {
		return ""
	}
	port := 8080
	if a.ruleManager != nil {
		port = a.GetListenPort()
	}
	if port <= 0 {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func (a *App) noticeSources() []noticeSource {
	sources := make([]noticeSource, 0, 3)
	if proxy := a.localProxyURL(); proxy != "" {
		sources = append(sources, noticeSource{name: "local-proxy", url: noticesEndpoint, proxy: proxy})
	}
	sources = append(sources, noticeSource{name: "direct", url: noticesEndpoint})
	sources = append(sources, noticeSource{name: "mirror", url: noticesMirrorEndpoint})
	return sources
}

func (a *App) fetchNotices() (*NoticesPayload, error) {
	failures := make([]string, 0, 3)
	for _, source := range a.noticeSources() {
		payload, err := a.fetchNoticeSource(source)
		if err == nil {
			return payload, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", source.name, err))
	}
	message := strings.Join(failures, " | ")
	if len(message) > 400 {
		message = message[:400] + "…"
	}
	return nil, fmt.Errorf("公告服务不可达（%s）", message)
}

func (a *App) fetchNoticeSource(source noticeSource) (*NoticesPayload, error) {
	transport := &http.Transport{
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: 6 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	if source.proxy != "" {
		proxyURL, err := url.Parse(source.proxy)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{Timeout: noticesTimeout, Transport: transport}
	defer transport.CloseIdleConnections()

	ctx, cancel := context.WithTimeout(context.Background(), noticesTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SniShaper/"+VersionString())
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	var envelope struct {
		OK        *bool    `json:"ok"`
		Version   int      `json:"version"`
		UpdatedAt string   `json:"updated_at"`
		Notices   []Notice `json:"notices"`
		Error     string   `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	if envelope.OK != nil && !*envelope.OK {
		message := envelope.Error
		if message == "" {
			message = "服务返回错误"
		}
		return nil, fmt.Errorf("%s", message)
	}
	notices := envelope.Notices
	if notices == nil {
		notices = []Notice{}
	}
	return &NoticesPayload{
		Version:   envelope.Version,
		UpdatedAt: envelope.UpdatedAt,
		Notices:   notices,
		FetchedAt: time.Now().Unix(),
		Source:    source.name,
	}, nil
}

func (a *App) noticesCachePath() string {
	dir := ""
	if a.settingsPath != "" {
		dir = filepath.Dir(a.settingsPath)
	}
	if dir == "" || dir == "." {
		if ep, err := os.Executable(); err == nil {
			dir = filepath.Join(filepath.Dir(ep), "config")
		}
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, noticesCacheName)
}

func (a *App) loadNoticesCache() *NoticesPayload {
	path := a.noticesCachePath()
	if path == "" {
		return nil
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var payload NoticesPayload
	if err := json.Unmarshal(blob, &payload); err != nil {
		return nil
	}
	if payload.Notices == nil {
		payload.Notices = []Notice{}
	}
	payload.Source = "disk-cache"
	return &payload
}

func (a *App) saveNoticesCache(payload *NoticesPayload) {
	path := a.noticesCachePath()
	if path == "" || payload == nil {
		return
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	_ = os.WriteFile(path, blob, 0644)
}
