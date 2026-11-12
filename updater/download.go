//go:build windows

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

const userAgent = "SniShaperUpdater/1.0"

type httpClient struct {
	client  *http.Client
	retries int
	logger  *Logger
	mirrors []string
}

func newHTTPClient(c Config, l *Logger) *httpClient {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       60 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	timeout := time.Duration(c.HTTPTimeoutMinutes) * time.Minute
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	return &httpClient{
		client:  &http.Client{Transport: transport, Timeout: timeout},
		retries: c.MaxRetries,
		logger:  l,
		mirrors: c.MirrorURLs,
	}
}

func (h *httpClient) urls(raw string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(u string) {
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	add(raw)
	for _, m := range h.mirrors {
		m = strings.TrimRight(strings.TrimSpace(m), "/")
		if m == "" {
			continue
		}
		add(m + "/" + raw)
	}
	return out
}

func (h *httpClient) pause(attempt int) {
	d := time.Duration(1<<attempt) * time.Second
	if d > 8*time.Second {
		d = 8 * time.Second
	}
	time.Sleep(d)
}

func (h *httpClient) get(ctx context.Context, raw string, limit int64) ([]byte, error) {
	var last error
	for _, u := range h.urls(raw) {
		for attempt := 0; attempt <= h.retries; attempt++ {
			b, err := h.getOnce(ctx, u, limit)
			if err == nil {
				return b, nil
			}
			last = err
			h.logger.Logf("GET failed: %s (%v)", u, err)
			if attempt < h.retries {
				h.pause(attempt)
			}
		}
	}
	if last == nil {
		last = fmt.Errorf("no download URL available")
	}
	return nil, last
}

func (h *httpClient) getOnce(ctx context.Context, u string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("response too large: %d bytes", resp.ContentLength)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return b, nil
}

func (h *httpClient) download(ctx context.Context, raw, dest string, wantSize int64) error {
	part := dest + ".part"
	var last error
	for _, u := range h.urls(raw) {
		for attempt := 0; attempt <= h.retries; attempt++ {
			if err := h.downloadAttempt(ctx, u, part); err != nil {
				last = err
				h.logger.Logf("download attempt failed: %s (%v)", u, err)
				if attempt < h.retries {
					h.pause(attempt)
				}
				continue
			}
			st, err := os.Stat(part)
			if err != nil {
				last = err
				continue
			}
			if wantSize > 0 && st.Size() != wantSize {
				last = fmt.Errorf("size mismatch: got %d, want %d", st.Size(), wantSize)
				_ = os.Remove(part)
				continue
			}
			if err := os.Rename(part, dest); err != nil {
				return fmt.Errorf("cannot finalize download: %w", err)
			}
			h.logger.Logf("downloaded %s (%d bytes)", dest, st.Size())
			return nil
		}
	}
	if last == nil {
		last = fmt.Errorf("no download URL available")
	}
	return last
}

func (h *httpClient) downloadAttempt(ctx context.Context, u, part string) error {
	offset := int64(0)
	if st, err := os.Stat(part); err == nil {
		offset = st.Size()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	switch {
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		_ = os.Remove(part)
		return fmt.Errorf("range not satisfiable, restart from zero")
	case resp.StatusCode == http.StatusPartialContent && offset > 0:
		cr := resp.Header.Get("Content-Range")
		if cr != "" && !strings.HasPrefix(cr, fmt.Sprintf("bytes %d-", offset)) {
			_ = os.Remove(part)
			return fmt.Errorf("unexpected Content-Range %q", cr)
		}
		flags = os.O_WRONLY | os.O_APPEND
	case resp.StatusCode == http.StatusOK:
		offset = 0
	default:
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.OpenFile(part, flags, 0644)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	h.logger.Logf("download chunk from %s: %d bytes (resume offset %d)", u, n, offset)
	return nil
}
