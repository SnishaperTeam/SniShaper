package proxy

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// startEchoListener 起一个只接受连接的本地 TCP 监听，返回其 host:port。
func startEchoListener(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	return ln.Addr().String()
}

// TestDialUpstreamDoHFallbackAfterDeadUpstream 复现 IDM 下载 GitHub Release 报
// 502 的根因：规则 upstream 指向一个不可达的硬编码地址。修复后主候选拨号失败
// 必须按 DoH 回退到目标域名本身的解析结果，而不是把失败直接抛成 502。
func TestDialUpstreamDoHFallbackAfterDeadUpstream(t *testing.T) {
	upstream := startEchoListener(t)
	host, port, err := net.SplitHostPort(upstream)
	if err != nil {
		t.Fatalf("split listener addr: %v", err)
	}

	p := NewProxyServer("127.0.0.1:8080")
	cr := &connectResult{
		effectiveMode: "direct",
		targetHost:    "localhost",
		targetAddr:    net.JoinHostPort("localhost", port),
		rule: Rule{
			Mode:     "direct",
			Enabled:  true,
			Upstream: net.JoinHostPort(host, "1"), // 死端口：主候选必然拨号失败
			DNSMode:  "system",                    // 测试环境无 DoH 节点，回退走系统解析
		},
	}

	if err := p.dialUpstream(cr); err != nil {
		t.Fatalf("dialUpstream should succeed via DoH fallback, got: %v", err)
	}
	if cr.conn == nil {
		t.Fatal("dialUpstream left cr.conn nil")
	}
	_ = cr.conn.Close()
}

// TestDialUpstreamFallbackExcludesTriedCandidates 保证回退候选不会重复尝试
// 主阶段已经失败过的地址。
func TestBuildDoHFallbackCandidatesExcludesTried(t *testing.T) {
	upstream := startEchoListener(t)
	_, port, _ := net.SplitHostPort(upstream)

	p := NewProxyServer("127.0.0.1:8080")
	cr := &connectResult{
		effectiveMode: "direct",
		targetHost:    "localhost",
		targetAddr:    net.JoinHostPort("localhost", port),
		rule:          Rule{Mode: "direct", Enabled: true, DNSMode: "system"},
	}
	tried := []string{net.JoinHostPort("127.0.0.1", port)}
	fallback := p.buildDoHFallbackCandidates(cr, tried)
	for _, addr := range fallback {
		if addr == tried[0] {
			t.Fatalf("fallback re-tries already-failed candidate %s: %v", addr, fallback)
		}
	}
}

// TestDialUpstreamNoFallbackForLiteralTarget 目标本身是字面 IP 时没有可回退的
// 解析，dialUpstream 必须把拨号错误原样返回（含失败候选与原因），供上层回
// 502 并写入日志。
func TestDialUpstreamNoFallbackForLiteralTarget(t *testing.T) {
	p := NewProxyServer("127.0.0.1:8080")
	cr := &connectResult{
		effectiveMode: "direct",
		targetHost:    "127.0.0.1",
		targetAddr:    net.JoinHostPort("127.0.0.1", "1"),
		rule:          Rule{Mode: "direct", Enabled: true},
	}
	err := p.dialUpstream(cr)
	if err == nil {
		t.Fatal("expected dial error for dead literal target")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Fatalf("error should name the failed candidate, got: %v", err)
	}
	if cr.conn != nil {
		t.Fatal("cr.conn must stay nil on failure")
	}
}

// TestBuiltinRulesCoverDownloadDomains 内置规则必须覆盖 GitHub Release、GHCR、
// WSL 更新与 Windows Update 的下载域名，否则这些流量会落进默认 direct 并在
// 上游不可达时对客户端回 502。
func TestBuiltinRulesCoverDownloadDomains(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "rules", "config.json"))
	if err != nil {
		t.Skipf("rules/config.json not available: %v", err)
	}
	var cfg RulesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse rules/config.json: %v", err)
	}

	var domains []string
	for _, sg := range cfg.SiteGroups {
		if !sg.Enabled {
			continue
		}
		domains = append(domains, sg.Domains...)
	}

	critical := []string{
		"github.com",
		"api.github.com",
		"codeload.github.com",
		"objects.githubusercontent.com",
		"release-assets.githubusercontent.com",
		"github-releases.githubusercontent.com",
		"pkg-containers.githubusercontent.com",
		"raw.githubusercontent.com",
		"ghcr.io",
		"wslstorestorage.blob.core.windows.net",
		"fg.download.windowsupdate.com",
		"fe3.delivery.mp.microsoft.com",
	}
	for _, host := range critical {
		best := -1
		for _, d := range domains {
			if s := domainMatchScore(host, d); s > best {
				best = s
			}
		}
		if best < 0 {
			t.Errorf("no enabled builtin rule matches download domain %q", host)
		}
	}
}

// TestBuiltinGitHubAssetUpstreamsNotSingleIP GitHub 资产域名规则此前只配了单个
// 硬编码 IP，该 IP 一旦失效整条规则直接 502。要求这类规则的上游是多 IP 列表。
func TestBuiltinGitHubAssetUpstreamsNotSingleIP(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "rules", "config.json"))
	if err != nil {
		t.Skipf("rules/config.json not available: %v", err)
	}
	var cfg RulesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse rules/config.json: %v", err)
	}

	for _, sg := range cfg.SiteGroups {
		if !sg.Enabled || sg.Upstream == "" {
			continue
		}
		coversAsset := false
		for _, d := range sg.Domains {
			if domainMatchScore("objects.githubusercontent.com", d) >= 0 ||
				domainMatchScore("raw.githubusercontent.com", d) >= 0 {
				coversAsset = true
				break
			}
		}
		if !coversAsset {
			continue
		}
		if got := len(splitUpstreamCandidates("objects.githubusercontent.com", sg.Upstream, "443")); got < 2 {
			t.Errorf("site group %q covers GitHub asset domains but upstream has only %d candidate(s): %s",
				sg.Name, got, sg.Upstream)
		}
	}
}
