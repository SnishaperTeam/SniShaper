package evolution

import (
	"fmt"
	"strings"
	"time"
)

func GenerateRule(domain string, method TestMethod, sniFake string, echEnabled bool) *TempRule {
	rule := &TempRule{
		ID:         fmt.Sprintf("evolution-%s-%s", domain, method),
		Name:       extractDomainPrefix(domain),
		Domain:     domain,
		Method:     method,
		CreatedAt:  time.Now(),
		IsApplied:  false,
		ECHEnabled: echEnabled,
	}

	switch method {
	case MethodDomainFronting:
		rule.Mode = "mitm"
		rule.SniFake = sniFake
	case MethodTLSFragment:
		rule.Mode = "tls-rf"
	case MethodECH:
		rule.Mode = "mitm"
		rule.ECHEnabled = true
	case MethodQUIC:
		rule.Mode = "quic"
	}

	return rule
}

func extractDomainPrefix(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return domain
}

func (r *TempRule) ToSiteGroup() map[string]interface{} {
	siteGroup := map[string]interface{}{
		"id":      r.ID,
		"name":    r.Name,
		"website": inferWebsite(r.Domain),
		"domains": []string{r.Domain},
		"mode":    r.Mode,
		"enabled": true,
	}

	if r.SniFake != "" {
		siteGroup["sni_fake"] = r.SniFake
	}

	if r.ECHEnabled {
		siteGroup["ech_enabled"] = true
		siteGroup["ech_profile_id"] = "legacy-cloudflare"
	}

	if r.UseCFPool {
		siteGroup["use_cf_pool"] = true
	}

	return siteGroup
}

func inferWebsite(domain string) string {
	if contains(domain, "google") {
		return "Google"
	}
	if contains(domain, "github") {
		return "GitHub"
	}
	if contains(domain, "telegram") {
		return "Telegram"
	}
	if contains(domain, "twitter") || contains(domain, "x.com") {
		return "Twitter"
	}
	if contains(domain, "youtube") {
		return "YouTube"
	}
	if contains(domain, "facebook") || contains(domain, "fb.com") {
		return "Facebook"
	}
	if contains(domain, "instagram") {
		return "Instagram"
	}
	if contains(domain, "cloudflare") {
		return "Cloudflare"
	}
	if contains(domain, "amazon") || contains(domain, "aws") {
		return "Amazon"
	}
	if contains(domain, "microsoft") {
		return "Microsoft"
	}

	return "Others"
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr ||
		(len(s) > len(substr) && contains(s[1:], substr)))
}
