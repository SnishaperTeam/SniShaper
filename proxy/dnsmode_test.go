package proxy

import (
	"reflect"
	"testing"
)

// TestRuleRequiresIPv6 pins down which dns_mode / NAT64 combinations count as a
// hard address-family constraint. prefer_ipv6 is deliberately excluded: it
// prefers v6 but already falls back to v4, so it never needs the strict path.
func TestRuleRequiresIPv6(t *testing.T) {
	tests := []struct {
		name string
		rule Rule
		want bool
	}{
		{
			name: "ipv6_only is a hard constraint",
			rule: Rule{DNSMode: "ipv6_only"},
			want: true,
		},
		{
			name: "ipv6_only is case and space insensitive",
			rule: Rule{DNSMode: "  IPv6_Only "},
			want: true,
		},
		{
			name: "nat64 with a profile is a hard constraint",
			rule: Rule{NAT64Enabled: true, NAT64ProfileID: "profile-1"},
			want: true,
		},
		{
			name: "nat64 without a profile is inert",
			rule: Rule{NAT64Enabled: true, NAT64ProfileID: ""},
			want: false,
		},
		{
			name: "nat64 profile alone is not enough",
			rule: Rule{NAT64Enabled: false, NAT64ProfileID: "profile-1"},
			want: false,
		},
		{
			name: "prefer_ipv6 falls back and needs no strict path",
			rule: Rule{DNSMode: "prefer_ipv6"},
			want: false,
		},
		{
			name: "default needs no strict path",
			rule: Rule{},
			want: false,
		},
		{
			name: "ipv4_only is not an IPv6 constraint",
			rule: Rule{DNSMode: "ipv4_only"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ruleRequiresIPv6(tt.rule); got != tt.want {
				t.Fatalf("ruleRequiresIPv6(%+v) = %v, want %v", tt.rule, got, tt.want)
			}
		})
	}
}

// TestBuildDialCandidatesRejectsIPv6OnIPv4Only covers the regression this guards:
// buildDialCandidates used to fall back to the bare targetAddr when resolution
// produced nothing, which turned "no IPv6 here" into an endless connect timeout
// instead of a clear configuration error.
func TestBuildDialCandidatesRejectsIPv6OnIPv4Only(t *testing.T) {
	if hasUsableIPv6() {
		t.Skip("host has usable IPv6; the IPv4-only guard cannot be exercised here")
	}

	p := &ProxyServer{}
	tests := []struct {
		name string
		rule Rule
	}{
		{
			name: "ipv6_only yields no candidates",
			rule: Rule{DNSMode: "ipv6_only", Mode: "mitm"},
		},
		{
			name: "nat64 yields no candidates",
			rule: Rule{Mode: "mitm", NAT64Enabled: true, NAT64ProfileID: "profile-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.buildDialCandidates(t.Context(), "example.com", "example.com:443", tt.rule, "mitm")
			if len(got) != 0 {
				t.Fatalf("expected no candidates on IPv4-only host, got %v", got)
			}
		})
	}
}

// TestBuildDialCandidatesKeepsUnconstrainedRules guards the opposite direction:
// rules without an address-family constraint must not be affected by the guard.
func TestBuildDialCandidatesKeepsUnconstrainedRules(t *testing.T) {
	p := &ProxyServer{}
	for _, mode := range []string{"", "prefer_ipv4", "prefer_ipv6", "ipv4_only"} {
		t.Run("mode="+mode, func(t *testing.T) {
			got := p.buildDialCandidates(t.Context(), "example.invalid", "example.invalid:443", Rule{DNSMode: mode, Mode: "mitm"}, "mitm")
			if len(got) == 0 {
				t.Fatalf("mode %q must keep producing candidates", mode)
			}
		})
	}
}

func TestOrderIPsByDNSMode(t *testing.T) {
	tests := []struct {
		name    string
		ips     []string
		dnsMode string
		want    []string
	}{
		{
			name:    "default prefers v4",
			ips:     []string{"2606:4700:4700::1111", "1.1.1.1", "8.8.8.8"},
			dnsMode: "",
			want:    []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"},
		},
		{
			name:    "prefer_ipv4 prefers v4",
			ips:     []string{"2606:4700:4700::1111", "1.1.1.1"},
			dnsMode: "prefer_ipv4",
			want:    []string{"1.1.1.1", "2606:4700:4700::1111"},
		},
		{
			name:    "prefer_ipv6 puts v6 first",
			ips:     []string{"1.1.1.1", "2606:4700:4700::1111", "8.8.8.8"},
			dnsMode: "prefer_ipv6",
			want:    []string{"2606:4700:4700::1111", "1.1.1.1", "8.8.8.8"},
		},
		{
			name:    "prefer_ipv6 falls back to v4 when no v6",
			ips:     []string{"1.1.1.1"},
			dnsMode: "prefer_ipv6",
			want:    []string{"1.1.1.1"},
		},
		{
			name:    "ipv4_only filters v6",
			ips:     []string{"1.1.1.1", "2606:4700:4700::1111"},
			dnsMode: "ipv4_only",
			want:    []string{"1.1.1.1"},
		},
		{
			name:    "ipv6_only filters v4",
			ips:     []string{"1.1.1.1", "2606:4700:4700::1111"},
			dnsMode: "ipv6_only",
			want:    []string{"2606:4700:4700::1111"},
		},
		{
			name:    "ipv6_only empty when no v6",
			ips:     []string{"1.1.1.1"},
			dnsMode: "ipv6_only",
			want:    nil,
		},
		{
			name:    "garbage entries skipped",
			ips:     []string{"not-an-ip", "1.1.1.1"},
			dnsMode: "",
			want:    []string{"1.1.1.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orderIPsByDNSMode(tt.ips, tt.dnsMode)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("orderIPsByDNSMode(%v, %q) = %v, want %v", tt.ips, tt.dnsMode, got, tt.want)
			}
		})
	}
}
