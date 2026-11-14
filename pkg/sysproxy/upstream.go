package sysproxy

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// 软件内所有连接上游服务的 HTTP 请求（公告、更新检查/下载、规则更新等）
// 统一走系统代理地址：代理开启且可达时使用，否则退回环境变量语义（通常直连）。
// 可达判定：代理对探测请求返回了任何 HTTP 响应（200/403/404/502…）都算可用；
// 连接失败或超时才算不可达。
// 可用结果缓存 1 分钟；不可用结果只缓存 10 秒，避免代理瞬时不可达导致
// 长达 1 分钟的静默直连。

const (
	upstreamProbeTTL      = time.Minute
	upstreamProbeRetryTTL = 10 * time.Second
)

var (
	upstreamMu     sync.Mutex
	upstreamServer string
	upstreamURL    *url.URL
	upstreamOK     bool
	upstreamAt     time.Time
)

// UpstreamProxyFunc 返回供 http.Transport.Proxy 使用的代理函数。
func UpstreamProxyFunc() func(*http.Request) (*url.URL, error) {
	u := usableSystemProxy()
	if u == nil {
		return http.ProxyFromEnvironment
	}
	return proxyVia(u)
}

func proxyVia(u *url.URL) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		// 与系统 <local> 语义一致：回环目标不走代理，httptest 等本机地址不被劫持。
		if isLoopbackHost(req.URL.Hostname()) {
			return nil, nil
		}
		return u, nil
	}
}

// usableSystemProxy 返回探测通过的系统代理地址；系统代理未开启或不可达时返回 nil。
func usableSystemProxy() *url.URL {
	status := manager.Status()
	if !status.Enabled {
		return nil
	}
	u, err := parseProxyServer(status.Server)
	if err != nil {
		return nil
	}
	if cached, ok, fresh := cachedUpstream(status.Server, time.Now()); fresh {
		if ok {
			return cached
		}
		return nil
	}
	upstreamMu.Lock()
	upstreamServer, upstreamURL, upstreamOK, upstreamAt = status.Server, u, probeProxy(u), time.Now()
	ok := upstreamOK
	upstreamMu.Unlock()
	if ok {
		return u
	}
	return nil
}

// cachedUpstream 返回 server 的缓存探测结果：(代理地址, 是否可用, 是否命中)。
// 可用结果在 upstreamProbeTTL 内有效；不可用结果仅缓存 upstreamProbeRetryTTL，
// 代理瞬时不可达时能被快速重试，不会长时间静默直连。
func cachedUpstream(server string, now time.Time) (*url.URL, bool, bool) {
	upstreamMu.Lock()
	defer upstreamMu.Unlock()
	if upstreamServer != server {
		return nil, false, false
	}
	ttl := upstreamProbeRetryTTL
	if upstreamOK {
		ttl = upstreamProbeTTL
	}
	if now.Sub(upstreamAt) < ttl {
		return upstreamURL, upstreamOK, true
	}
	return nil, false, false
}

// probeProxy 只要代理对探测请求返回了任何 HTTP 响应就算可用。
func probeProxy(u *url.URL) bool {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(u),
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	defer client.CloseIdleConnections()
	resp, err := client.Head("http://www.msftconnecttest.com/connecttest.txt")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// parseProxyServer 兼容注册表里 "host:port" 与分协议写法
// "http=host:port;https=host:port"（优先取 https 段）。
func parseProxyServer(raw string) (*url.URL, error) {
	addr := strings.TrimSpace(raw)
	if strings.Contains(addr, "=") {
		addr = ""
		for _, key := range []string{"https=", "http="} {
			for _, seg := range strings.Split(raw, ";") {
				if v, ok := strings.CutPrefix(strings.TrimSpace(seg), key); ok && v != "" {
					addr = v
					break
				}
			}
			if addr != "" {
				break
			}
		}
		if addr == "" {
			return nil, fmt.Errorf("no http/https entry in %q", raw)
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("empty system proxy server")
	}
	return url.Parse("http://" + addr)
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
