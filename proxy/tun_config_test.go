package proxy

import (
	"net/netip"
	"testing"
)

func TestTUNConfigInterfaceConfigIgnoresRouteExcludes(t *testing.T) {
	cfg := TUNConfig{
		InterfaceName:         "Ethernet",
		ExcludeInterfaces:     []string{"Virtual*"},
		RouteExcludeAddresses: []string{"192.168.0.0/16"},
	}

	got := cfg.InterfaceConfig()

	if got.ForceInterface != "Ethernet" {
		t.Fatalf("ForceInterface must pass through, got %q", got.ForceInterface)
	}
	if len(got.ExcludeInterfaces) != 1 || got.ExcludeInterfaces[0] != "Virtual*" {
		t.Fatalf("ExcludeInterfaces must pass through, got %v", got.ExcludeInterfaces)
	}
	// route_exclude_address 是"不进 TUN 的路由"，与"不得作为出站网卡"语义不同。
	// 混用会让用户为放行内网而填的网段把唯一出口网卡直接排除掉。
	if len(got.ExcludeAddresses) != 0 {
		t.Fatalf("RouteExcludeAddresses must not feed NIC exclusion, got %v", got.ExcludeAddresses)
	}
}

func TestNormalizeTUNConfigClampsMTU(t *testing.T) {
	cases := []struct {
		in   int
		want int
	}{
		{0, 9000},
		{-1, 9000},
		{1, 576},
		{575, 576},
		{1500, 1500},
		{9000, 9000},
		{9001, 9000},
		{65535, 9000},
	}
	for _, tc := range cases {
		got := normalizeTUNConfig(TUNConfig{MTU: tc.in}).MTU
		if got != tc.want {
			t.Fatalf("normalizeTUNConfig(MTU=%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeTUNConfigTrimsAndDeduplicates(t *testing.T) {
	cfg := normalizeTUNConfig(TUNConfig{
		InterfaceName:         "  Ethernet  ",
		ExcludeInterfaces:     []string{" a ", "a", "", "b"},
		RouteExcludeAddresses: []string{" 10.0.0.0/8 ", "10.0.0.0/8"},
	})

	if cfg.InterfaceName != "Ethernet" {
		t.Fatalf("InterfaceName must be trimmed, got %q", cfg.InterfaceName)
	}
	if len(cfg.ExcludeInterfaces) != 2 {
		t.Fatalf("ExcludeInterfaces must be trimmed and deduplicated, got %v", cfg.ExcludeInterfaces)
	}
	if len(cfg.RouteExcludeAddresses) != 1 {
		t.Fatalf("RouteExcludeAddresses must be deduplicated, got %v", cfg.RouteExcludeAddresses)
	}
}

func TestNormalizeTUNConfigDisablesStrictRoute(t *testing.T) {
	if normalizeTUNConfig(TUNConfig{StrictRoute: true}).StrictRoute {
		t.Fatal("StrictRoute must stay disabled: it blackholes traffic under Wintun")
	}
}

func TestRouteExcludePrefixesClassifiesFamilies(t *testing.T) {
	cfg := TUNConfig{
		RouteExcludeAddresses: []string{
			"10.0.0.0/8",
			"fd00::/8",
			"192.168.1.1",
			"not-a-cidr",
			"",
		},
	}

	v4, v6 := cfg.RouteExcludePrefixes()

	if len(v4) != 2 {
		t.Fatalf("expected 2 IPv4 prefixes, got %v", v4)
	}
	if len(v6) != 1 {
		t.Fatalf("expected 1 IPv6 prefix, got %v", v6)
	}
	if v6[0].String() != "fd00::/8" {
		t.Fatalf("unexpected IPv6 prefix %s", v6[0])
	}
	// 裸地址应被规范化为 /32 与 /128。
	var foundHost, foundLAN bool
	for _, p := range v4 {
		if p.String() == "192.168.1.1/32" {
			foundHost = true
		}
		if p.Contains(netip.MustParseAddr("10.1.2.3")) {
			foundLAN = true
		}
	}
	if !foundHost || !foundLAN {
		t.Fatalf("unexpected IPv4 prefixes %v", v4)
	}
}
