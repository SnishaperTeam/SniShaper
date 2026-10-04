package singtun

import "strings"

// fakeIPBypassDomains are resolved for real instead of being handed a fake-ip.
//
// The Windows connectivity probe is the important case. NCSI resolves
// dns.msftncsi.com and treats the answer as evidence of reachability; the
// fake-ip range 198.18.0.0/15 is reserved for benchmarking (RFC 2544) and is
// not a public address, so Windows rejects it and marks the adapter as having
// no Internet access without ever sending the HTTP probe. Clash, mihomo and
// sing-box all ship the same exclusion for this reason.
//
// Resolving these for real also keeps STUN and mip6 style traffic working,
// which legitimately needs the genuine address.
var fakeIPBypassDomains = []string{
	// Windows NCSI.
	"dns.msftncsi.com",
	"www.msftconnecttest.com",
	"www.msftncsi.com",
	"msftncsi.com",
	"msftconnecttest.com",
	// Windows connectivity suffix, covers regional variants.
	"+.msft.nettest.org",
	"+.msftncsi.com",
	"+.msftconnecttest.com",
	// STUN needs the real address to compute the reflexive candidate.
	"stun.l.google.com",
	"stun.cloudflare.com",
	"+.stun.*.com",
	"+.stun.*.net",
	"+.stun.*.org",
}

// fakeIPBypassSuffixes are matched as suffixes of the queried name.
var fakeIPBypassSuffixes = []string{
	".msftncsi.com",
	".msftconnecttest.com",
	".msft.net",
	".nettest.org",
}

// shouldBypassFakeIP reports whether a domain must be resolved for real.
func shouldBypassFakeIP(domain string) bool {
	name := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))
	if name == "" {
		return false
	}

	for _, entry := range fakeIPBypassDomains {
		if entry == name {
			return true
		}
		// "+." prefix means "this domain and anything under it".
		if strings.HasPrefix(entry, "+.") {
			suffix := entry[1:] // keep the leading dot
			if strings.HasSuffix(name, suffix) {
				return true
			}
			// ".msftncsi.com" should also match the bare "msftncsi.com".
			if name == suffix[1:] {
				return true
			}
		}
	}

	for _, suffix := range fakeIPBypassSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}
