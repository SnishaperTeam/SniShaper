// Package rulesguard 存放 rules/config.json 的数据守卫测试：内置规则数据必须
// 继续覆盖关键下载域名，且 GitHub 资产域名规则走 MITM 直连官方 IP。
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

// TestBuiltinGitHubAssetsUseMITMOfficialIP GitHub 资产域名规则改用官方 IP 直连
// 并走 MITM，不再经腾讯 EdgeOne 反代（106.55.101.131）。要求这类规则模式为
// mitm 且上游首位属于官方 IP 列表。
func TestBuiltinGitHubAssetsUseMITMOfficialIP(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "rules", "config.json"))
	if err != nil {
		t.Skipf("rules/config.json not available: %v", err)
	}
	var cfg proxy.RulesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse rules/config.json: %v", err)
	}

	official := map[string]bool{
		"20.205.243.168": true,
		"185.199.110.154": true,
		"185.199.108.133": true,
		"140.82.113.18":  true,
		"140.82.114.17":  true,
		"20.205.243.165": true,
		"20.27.177.113":  true,
		"140.82.113.22":  true,
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
		if sg.Mode != "mitm" {
			t.Errorf("site group %q covers GitHub asset domains but mode is %q, want mitm", sg.Name, sg.Mode)
		}
		first := firstUpstreamCandidate(sg.Upstream)
		host, _, err := net.SplitHostPort(first)
		if err != nil || !official[host] {
			t.Errorf("site group %q covers GitHub asset domains but upstream primary is %q, want official GitHub IP",
				sg.Name, first)
		}
	}
}
