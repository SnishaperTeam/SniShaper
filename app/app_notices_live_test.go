package app

import (
	"os"
	"testing"
)

func TestFetchNoticesLive(t *testing.T) {
	if os.Getenv("SNISHAPER_LIVE_NOTICE_TEST") != "1" {
		t.Skip("set SNISHAPER_LIVE_NOTICE_TEST=1 to run")
	}
	a := &App{}
	source := noticeSource{name: "live", url: noticesEndpoint, proxy: os.Getenv("SNISHAPER_NOTICE_TEST_PROXY")}
	payload, err := a.fetchNoticeSource(source)
	if err != nil {
		t.Fatalf("live fetch failed: %v", err)
	}
	if payload.Version <= 0 || len(payload.Notices) == 0 {
		t.Fatalf("unexpected payload: version=%d count=%d", payload.Version, len(payload.Notices))
	}
	t.Logf("version=%d notices=%d first=%q", payload.Version, len(payload.Notices), payload.Notices[0].Title)
}
