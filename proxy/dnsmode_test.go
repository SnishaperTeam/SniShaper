package proxy

import (
	"reflect"
	"testing"
)

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
