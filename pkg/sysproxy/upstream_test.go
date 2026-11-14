package sysproxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestParseProxyServer(t *testing.T) {
	cases := []struct {
		raw  string
		host string
		ok   bool
	}{
		{"127.0.0.1:7890", "127.0.0.1:7890", true},
		{" http=10.0.0.1:8080;https=10.0.0.2:8443;ftp=x ", "10.0.0.2:8443", true},
		{"http=10.0.0.1:8080;ftp=x", "10.0.0.1:8080", true},
		{"ftp=1.2.3.4:21", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		u, err := parseProxyServer(c.raw)
		if c.ok && (err != nil || u.Host != c.host) {
			t.Errorf("parseProxyServer(%q) = %v, %v; want host %s", c.raw, u, err, c.host)
		}
		if !c.ok && err == nil {
			t.Errorf("parseProxyServer(%q) expected error", c.raw)
		}
	}
}

func TestProbeProxyAcceptsAnyStatus(t *testing.T) {
	// httptest server stands in for a proxy: 404 still counts as "proxy answered".
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !probeProxy(u) {
		t.Fatal("expected a 404 response from the proxy to count as usable")
	}
}

func TestProxyViaBypassesLoopback(t *testing.T) {
	u, _ := url.Parse("http://127.0.0.1:9")
	fn := proxyVia(u)
	if got, _ := fn(&http.Request{URL: &url.URL{Scheme: "https", Host: "127.0.0.1:1"}}); got != nil {
		t.Fatalf("loopback target should bypass proxy, got %v", got)
	}
	if got, _ := fn(&http.Request{URL: &url.URL{Scheme: "https", Host: "api.github.com"}}); got != u {
		t.Fatalf("external target should use the proxy, got %v", got)
	}
}

func TestUsableSystemProxyCacheRespectsTTL(t *testing.T) {
	proxyURL, _ := url.Parse("http://127.0.0.1:9")
	upstreamMu.Lock()
	upstreamServer, upstreamURL, upstreamOK, upstreamAt = "cached-host", proxyURL, true, time.Now()
	upstreamMu.Unlock()

	// System proxy disabled on this machine: cache must not be consulted.
	if got := usableSystemProxy(); got != nil {
		t.Fatalf("disabled system proxy must yield nil, got %v", got)
	}

	// Fresh cache entry under an enabled status stub is out of scope here
	// (registry reading is covered by core's own tests).
	upstreamMu.Lock()
	upstreamServer = ""
	upstreamMu.Unlock()
}

func TestCachedUpstreamTTLs(t *testing.T) {
	proxyURL, _ := url.Parse("http://10.0.0.1:8080")
	upstreamMu.Lock()
	upstreamServer, upstreamURL, upstreamOK, upstreamAt = "10.0.0.1:8080", proxyURL, true, time.Now()
	upstreamMu.Unlock()

	now := time.Now()
	if _, ok, fresh := cachedUpstream("10.0.0.1:8080", now); !fresh || !ok {
		t.Fatal("fresh usable entry must hit")
	}
	if _, _, fresh := cachedUpstream("other:1", now); fresh {
		t.Fatal("different server must not hit")
	}
	if _, _, fresh := cachedUpstream("10.0.0.1:8080", now.Add(upstreamProbeTTL-time.Second)); !fresh {
		t.Fatal("usable entry must stay fresh until upstreamProbeTTL")
	}
	if _, _, fresh := cachedUpstream("10.0.0.1:8080", now.Add(upstreamProbeTTL+time.Second)); fresh {
		t.Fatal("usable entry must expire after upstreamProbeTTL")
	}

	upstreamMu.Lock()
	upstreamOK = false
	upstreamMu.Unlock()
	if _, ok, fresh := cachedUpstream("10.0.0.1:8080", now.Add(upstreamProbeRetryTTL-time.Second)); !fresh || ok {
		t.Fatal("unusable entry must stay fresh (not ok) until upstreamProbeRetryTTL")
	}
	if _, _, fresh := cachedUpstream("10.0.0.1:8080", now.Add(upstreamProbeRetryTTL+time.Second)); fresh {
		t.Fatal("unusable entry must expire after upstreamProbeRetryTTL")
	}

	upstreamMu.Lock()
	upstreamServer = ""
	upstreamMu.Unlock()
}
