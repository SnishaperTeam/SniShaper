package app

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNoticesCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	a := &App{settingsPath: filepath.Join(dir, "settings.json")}
	payload := &NoticesPayload{
		Version:   7,
		UpdatedAt: "2026-11-08T00:00:00Z",
		Notices:   []Notice{{ID: "n1", Title: "标题", Body: "正文", Level: "info"}},
		FetchedAt: 1700000000,
		Source:    "network",
	}
	a.saveNoticesCache(payload)

	got := a.loadNoticesCache()
	if got == nil {
		t.Fatal("expected cache to load")
	}
	if got.Version != 7 || len(got.Notices) != 1 || got.Notices[0].ID != "n1" {
		t.Fatalf("unexpected cache payload: %+v", got)
	}
	if got.Source != "disk-cache" {
		t.Fatalf("expected disk-cache source, got %q", got.Source)
	}
}

func TestGetNoticesServesMemoryCache(t *testing.T) {
	a := &App{settingsPath: filepath.Join(t.TempDir(), "settings.json")}
	a.noticesPayload = &NoticesPayload{Version: 2, Notices: []Notice{{ID: "n1", Title: "t", Body: "b"}}}
	a.noticesFetched = time.Now()

	got, err := a.GetNotices(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Cached || got.Stale {
		t.Fatalf("expected fresh memory cache, got cached=%v stale=%v", got.Cached, got.Stale)
	}
	if got.Version != 2 || len(got.Notices) != 1 {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestNoticesCachePathFallback(t *testing.T) {
	a := &App{}
	if path := a.noticesCachePath(); path == "" {
		t.Fatal("expected a fallback cache path")
	}
}
