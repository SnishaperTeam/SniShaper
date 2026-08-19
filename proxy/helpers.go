package proxy

import (
	"net"
	"path/filepath"
	"strings"

	"snishaper/pkg/dohresolver"
)

// gfwListCachePath returns the path of the local GFW list cache file,
// relative to the rules file.
func gfwListCachePath(rulesPath string) string {
	return filepath.Join(filepath.Dir(rulesPath), "gfwlist_cache.txt")
}

func normalizeHost(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if hostport == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(hostport)
	if err == nil {
		// 去掉 DNS FQDN 格式的尾点（perplexity.ai. → perplexity.ai）
		// 否则 domainMatchScore 会因尾点不匹配规则
		return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	}

	if strings.HasPrefix(hostport, "[") && strings.HasSuffix(hostport, "]") {
		return strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(hostport, "["), "]"))
	}
	// 同上去尾点
	return strings.ToLower(strings.TrimSuffix(hostport, "."))
}

func ensureAddrWithPort(addr, defaultPort string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}

	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		if port == "" {
			port = defaultPort
		}
		return net.JoinHostPort(host, port)
	}

	if strings.HasPrefix(addr, "[") && strings.HasSuffix(addr, "]") {
		return net.JoinHostPort(strings.TrimSuffix(strings.TrimPrefix(addr, "["), "]"), defaultPort)
	}
	return net.JoinHostPort(addr, defaultPort)
}

func isLiteralIP(host string) bool {
	return net.ParseIP(strings.Trim(host, "[]")) != nil
}

func dedupeDialCandidates(candidates []string) []string {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func toProxyCertVerify(c dohresolver.CertVerifyConfig) CertVerifyConfig {
	return CertVerifyConfig{
		Mode:                  c.Mode,
		Names:                 c.Names,
		Suffixes:              c.Suffixes,
		SPKISHA256:            c.SPKISHA256,
		AllowUnknownAuthority: c.AllowUnknownAuthority,
	}
}

func toDohCertVerify(c CertVerifyConfig) dohresolver.CertVerifyConfig {
	return dohresolver.CertVerifyConfig{
		Mode:                  c.Mode,
		Names:                 c.Names,
		Suffixes:              c.Suffixes,
		SPKISHA256:            c.SPKISHA256,
		AllowUnknownAuthority: c.AllowUnknownAuthority,
	}
}

func normalizeDNSMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "system":
		return ""
	case "prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return ""
	}
}
