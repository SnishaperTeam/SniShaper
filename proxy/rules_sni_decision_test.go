package proxy

import (
	"strings"
	"testing"
)

// ---- SNIRewriteDecisionForTUN 用例（T015） ----
//
// 决策语义（US4）：
//   - matched=true  表示命中了启用的规则（含 ~ 正则 / 后缀域名），
//     此时 rewriteTo 为规则的 sni_fake（可被 TrimSpace，空串=只匹配不重写）
//   - matched=false 表示未命中 / 规则被禁用 / 正则非法 / rules 未初始化

func newDecisionServer(rules []Rule) *ProxyServer {
	p := NewProxyServer("")
	rm := NewRuleManager("", "")
	rm.SetRules(rules)
	p.SetRuleManager(rm)
	return p
}

// 用例 1：精确/后缀域名命中且配置了 sni_fake → 重写目标即 sni_fake。
func TestSNIRewriteDecisionSuffixMatch(t *testing.T) {
	p := newDecisionServer([]Rule{
		{Domain: "example.com", Mode: "transparent", SniFake: "  cdn.example.net ", Enabled: true},
	})
	rewriteTo, matched := p.SNIRewriteDecisionForTUN("www.example.com")
	if !matched {
		t.Fatal("suffix rule must match subdomain host")
	}
	if rewriteTo != "cdn.example.net" {
		t.Fatalf("expected trimmed sni_fake, got %q", rewriteTo)
	}
}

// 用例 2：~ 正则规则命中（复用 domainRegexCache 的 ~ 语法）。
func TestSNIRewriteDecisionRegexMatch(t *testing.T) {
	p := newDecisionServer([]Rule{
		{Domain: `~^blocked-\d+\.example\.com$`, Mode: "transparent", SniFake: "relay.example.org", Enabled: true},
	})
	if rewriteTo, matched := p.SNIRewriteDecisionForTUN("blocked-42.example.com"); !matched || rewriteTo != "relay.example.org" {
		t.Fatalf("regex rule must match, got rewriteTo=%q matched=%v", rewriteTo, matched)
	}
	if _, matched := p.SNIRewriteDecisionForTUN("other.example.com"); matched {
		t.Fatal("non-matching host must not match regex rule")
	}
}

// 用例 3：非法正则（~[）必须安全跳过，不得 panic。
func TestSNIRewriteDecisionInvalidRegex(t *testing.T) {
	p := newDecisionServer([]Rule{
		{Domain: `~[`, Mode: "transparent", SniFake: "x.example.org", Enabled: true},
	})
	if _, matched := p.SNIRewriteDecisionForTUN("anything.example.com"); matched {
		t.Fatal("invalid regex rule must be skipped")
	}
}

// 用例 4：命中但未配置 sni_fake → matched=true、rewriteTo 为空（只匹配不重写）。
func TestSNIRewriteDecisionMatchWithoutSniFake(t *testing.T) {
	p := newDecisionServer([]Rule{
		{Domain: "example.com", Mode: "direct", Enabled: true},
	})
	rewriteTo, matched := p.SNIRewriteDecisionForTUN("example.com")
	if !matched {
		t.Fatal("rule without sni_fake must still report matched=true")
	}
	if rewriteTo != "" {
		t.Fatalf("expected empty rewriteTo, got %q", rewriteTo)
	}
}

// 用例 5：未命中任何规则 → matched=false（默认 fallback 与真实规则的区分）。
func TestSNIRewriteDecisionNoMatch(t *testing.T) {
	p := newDecisionServer([]Rule{
		{Domain: "example.com", Mode: "transparent", SniFake: "x.example.org", Enabled: true},
	})
	if _, matched := p.SNIRewriteDecisionForTUN("unrelated.org"); matched {
		t.Fatal("unrelated host must not match")
	}
}

// 用例 6：Enabled=false 的规则不参与匹配。
func TestSNIRewriteDecisionDisabledRule(t *testing.T) {
	p := newDecisionServer([]Rule{
		{Domain: "example.com", Mode: "transparent", SniFake: "x.example.org", Enabled: false},
	})
	if _, matched := p.SNIRewriteDecisionForTUN("www.example.com"); matched {
		t.Fatal("disabled rule must not match")
	}
}

// 用例 7：rules 未初始化（NewProxyServer 后未 SetRuleManager）必须安全返回。
func TestSNIRewriteDecisionNilRuleManager(t *testing.T) {
	p := NewProxyServer("")
	if _, matched := p.SNIRewriteDecisionForTUN("example.com"); matched {
		t.Fatal("nil rule manager must yield matched=false")
	}
}

// 用例 8：长 SNI 重写目标透传（重写器自身限制 255，决策层不截断）。
func TestSNIRewriteDecisionPassesThroughLongSniFake(t *testing.T) {
	long := strings.Repeat("a", 200) + ".example.org"
	p := newDecisionServer([]Rule{
		{Domain: "example.com", Mode: "transparent", SniFake: long, Enabled: true},
	})
	if rewriteTo, matched := p.SNIRewriteDecisionForTUN("example.com"); !matched || rewriteTo != long {
		t.Fatalf("decision layer must pass sni_fake through, got %q matched=%v", rewriteTo, matched)
	}
}
