// Package rulesguard 存放 rules/config.json 的数据守卫测试：内置规则数据必须
// 继续覆盖关键下载域名，且 GitHub 资产域名规则的上游保持腾讯反代兜底。
// 匹配语义复用 snishaper-core 导出的 proxy.MatchDomainScore，保证与数据面一致。
package rulesguard

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SnishaperTeam/snishaper-core/proxy"
)

// firstUpstreamCandidate 取 upstream 字符串的第一个候选地址（缺端口时补 443，
// 与数据面 splitUpstreamCandidates 的 ensureAddrWithPort 行为一致）。
func firstUpstreamCandidate(upstream string) string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(upstream, "，", ","), ";", ",")
	for _, part := range strings.Split(normalized, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(trimmed); err != nil {
			return net.JoinHostPort(trimmed, "443")
		}
		return trimmed
	}
	return ""
}

// TestBuiltinRulesCoverDownloadDomains 内置规则必须覆盖 GitHub Release、GHCR、
// WSL 更新与 Windows Update 的下载域名，否则这些流量会落进默认 direct 并在
// 上游不可达时对客户端回 502。
func TestBuiltinRulesCoverDownloadDomains(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "rules", "config.json"))
	if err != nil {
		t.Skipf("rules/config.json not available: %v", err)
	}
	var cfg proxy.RulesConfig
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
			if s := proxy.MatchDomainScore(host, d); s > best {
				best = s
			}
		}
		if best < 0 {
			t.Errorf("no enabled builtin rule matches download domain %q", host)
		}
	}
}

// TestBuiltinGitHubAssetsUseTencentReverseProxy GitHub 资产域名规则经腾讯
// EdgeOne 反代（106.55.101.131）兜底，实测该反代比直连 Fastly anycast 更稳。
// 要求这类规则的上游首位必须是腾讯反代。
func TestBuiltinGitHubAssetsUseTencentReverseProxy(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "rules", "config.json"))
	if err != nil {
		t.Skipf("rules/config.json not available: %v", err)
	}
	var cfg proxy.RulesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse rules/config.json: %v", err)
	}

	for _, sg := range cfg.SiteGroups {
		if !sg.Enabled || sg.Upstream == "" {
			continue
		}
		coversAsset := false
		for _, d := range sg.Domains {
			if proxy.MatchDomainScore("objects.githubusercontent.com", d) >= 0 ||
				proxy.MatchDomainScore("raw.githubusercontent.com", d) >= 0 {
				coversAsset = true
				break
			}
		}
		if !coversAsset {
			continue
		}
		first := firstUpstreamCandidate(sg.Upstream)
		if host, _, err := net.SplitHostPort(first); err != nil || host != "106.55.101.131" {
			t.Errorf("site group %q covers GitHub asset domains but upstream primary is %q, want Tencent reverse proxy 106.55.101.131",
				sg.Name, first)
		}
	}
}
