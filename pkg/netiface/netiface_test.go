package netiface

import (
	"net/netip"
	"strings"
	"testing"
)

func testViews() []InterfaceView {
	return []InterfaceView{
		{
			Name:      "Loopback Pseudo-Interface 1",
			Index:     1,
			Up:        true,
			Loopback:  true,
			Addresses: []netip.Addr{netip.MustParseAddr("127.0.0.1"), netip.MustParseAddr("::1")},
		},
		{
			Name:      "vEthernet (Default Switch)",
			Index:     12,
			Up:        true,
			Addresses: []netip.Addr{netip.MustParseAddr("172.21.128.1")},
		},
		{
			Name:      "vEthernet (WSL (Hyper-V firewall))",
			Index:     34,
			Up:        true,
			Addresses: []netip.Addr{netip.MustParseAddr("172.30.192.1")},
		},
		{
			Name:      "VMware Network Adapter VMnet8",
			Index:     7,
			Up:        true,
			Addresses: []netip.Addr{netip.MustParseAddr("192.168.226.1")},
		},
		{
			Name:      "WLAN",
			Index:     9,
			Up:        true,
			Addresses: []netip.Addr{netip.MustParseAddr("192.168.1.42"), netip.MustParseAddr("2409:8a00:1234::42")},
		},
		{
			Name:      "Ethernet",
			Index:     14,
			Up:        true,
			Addresses: []netip.Addr{netip.MustParseAddr("10.0.0.8")},
		},
		{
			Name:      "SniShaper",
			Index:     55,
			Up:        true,
			Addresses: []netip.Addr{netip.MustParseAddr("198.18.0.1")},
		},
		{
			Name:      "Ethernet 2",
			Index:     18,
			Up:        false,
			Addresses: []netip.Addr{netip.MustParseAddr("192.168.50.9")},
		},
	}
}

func testRoutes() []defaultRoute {
	return []defaultRoute{
		{interfaceIndex: 12, gateway: netip.MustParseAddr("172.21.128.1"), metric: 5, hasMetric: true, disableDefault: true},
		{interfaceIndex: 9, gateway: netip.MustParseAddr("192.168.1.1"), metric: 40, hasMetric: true},
		{interfaceIndex: 34, gateway: netip.MustParseAddr("172.30.192.1"), metric: 5000, hasMetric: true},
	}
}

func stubResolver(t *testing.T, routes []defaultRoute, probe func(Candidate) error) {
	t.Helper()
	originalRoutes, originalProbe := lookupRoutes, probeCandidateRoute
	lookupRoutes = func(int) []defaultRoute { return routes }
	probeCandidateRoute = probe
	t.Cleanup(func() {
		lookupRoutes = originalRoutes
		probeCandidateRoute = originalProbe
	})
}

func noProbe(Candidate) error { return nil }

func TestVirtualAdapterClassification(t *testing.T) {
	virtual := []string{
		"vEthernet (Default Switch)",
		"vEthernet (WSL (Hyper-V firewall))",
		"Hyper-V Virtual Ethernet Adapter",
		"VMware Network Adapter VMnet1",
		"VMware Network Adapter VMnet8",
		"VirtualBox Host-Only Network",
		"docker0",
		"veth1a2b3c",
		"br-9f2c1d",
		"WireGuard Tunnel",
		"Tailscale",
		"OpenVPN TAP-Windows6",
		"OpenVPN Wintun",
		"Wintun Userspace Tunnel",
		"utun3",
		"Cloudflare WARP",
		"CloudflareWARP",
		"Npcap Loopback Adapter",
		"SniShaper",
		"Microsoft Wi-Fi Direct Virtual Adapter",
	}
	for _, name := range virtual {
		if !IsVirtualAdapter(name) {
			t.Errorf("expected %q to be treated as virtual", name)
		}
	}

	physical := []string{"WLAN", "以太网", "Ethernet", "Ethernet 2", "Wi-Fi", "en0", "eth0", "enp3s0", "wlan0", "Local Area Connection"}
	for _, name := range physical {
		if IsVirtualAdapter(name) {
			t.Errorf("expected %q to be treated as physical", name)
		}
	}
}

func TestProxyVendorAdaptersAreBlacklisted(t *testing.T) {
	cases := []string{
		"Meta",
		"Mihomo",
		"Clash",
		"utun0",
		"utun4",
		"sing-box",
		"singbox0",
		"singtun0",
		"tun0",
		"v2rayN",
		"Nekoray",
		"wg0",
		"WireGuard Tunnel",
		"tailscale0",
		"Tailscale",
		"zt0",
		"ZeroTier One [1a2b3c4d5e6f7788]",
		"tap0",
		"warp0",
		"Cisco AnyConnect Secure Mobility Client Virtual Miniport Adapter for Windows x64",
		"Fortinet SSL VPN Virtual Ethernet Adapter",
		"Sangfor SSL VPN CS Support System VNIC",
		"VPN Client Adapter",
	}
	for _, name := range cases {
		if !IsVirtualAdapter(name) {
			t.Errorf("expected proxy/VPN adapter %q to be blacklisted", name)
		}
	}
}

func TestSystemAndContainerAdaptersAreBlacklisted(t *testing.T) {
	linux := []string{
		"lo",
		"docker0",
		"br-1a2b3c4d5e6f",
		"veth0",
		"vetha1b2c3",
		"virbr0",
		"virbr0-nic",
		"vnet0",
		"vmnet1",
		"vmnet8",
		"vboxnet0",
		"ppp0",
		"bond0",
		"br0",
		"dummy0",
		"cni0",
		"flannel.1",
		"cali1234abc",
		"cilium_host",
		"kube-ipvs0",
		"tunl0",
		"vxlan.calico",
	}
	windows := []string{
		"vEthernet (Default Switch)",
		"vEthernet (WSL)",
		"VMware Network Adapter VMnet1",
		"VMware Network Adapter VMnet8",
		"VirtualBox Host-Only Network",
		"Microsoft KM-TEST 环回适配器",
		"Npcap Loopback Adapter",
	}
	darwin := []string{
		"lo0",
		"gif0",
		"stf0",
		"bridge0",
		"utun1",
		"awdl0",
		"llw0",
		"p2p0",
		"vmenet0",
		"vmnet1",
		"vmnet8",
	}
	for _, group := range [][]string{linux, windows, darwin} {
		for _, name := range group {
			if !IsVirtualAdapter(name) {
				t.Errorf("expected system/container adapter %q to be blacklisted", name)
			}
		}
	}
}

func TestBlacklistDoesNotMatchPhysicalAdapterNames(t *testing.T) {
	physical := []string{
		"WLAN",
		"以太网",
		"以太网 2",
		"Ethernet",
		"Wi-Fi",
		"Local Area Connection",
		"Local Area Connection* 10",
		"en0",
		"en1",
		"eth0",
		"enp3s0",
		"eno1",
		"wlp2s0",
		"wlan0",
		"wwan0",
		"enx001122334455",
	}
	for _, name := range physical {
		if IsVirtualAdapter(name) {
			t.Errorf("physical adapter %q must not be blacklisted", name)
		}
	}
}

func TestBluetoothAdaptersAreBlacklisted(t *testing.T) {
	for _, name := range []string{"Bluetooth Device (Personal Area Network)", "蓝牙网络连接"} {
		if !IsVirtualAdapter(name) {
			t.Errorf("bluetooth adapter %q must be blacklisted", name)
		}
	}
}

func TestOwnTunnelMarkers(t *testing.T) {
	for _, name := range []string{"SniShaper", "sing-box", "singbox0", "singtun0", "wintun0"} {
		if !IsOwnTunnel(name) {
			t.Fatalf("%q must be recognised as a tunnel owned by this process", name)
		}
	}
	for _, name := range []string{"WLAN", "Ethernet", "utun3"} {
		if IsOwnTunnel(name) {
			t.Fatalf("%q must not be recognised as own tunnel", name)
		}
	}
}

func TestCollectDropsTunInterfaceAndLoopback(t *testing.T) {
	candidates := collect(FamilyIPv4, Config{}.normalize(), testViews(), testRoutes())
	for _, candidate := range candidates {
		if candidate.InterfaceName == "SniShaper" {
			t.Fatal("TUN interface must never be an outbound candidate")
		}
		if strings.HasPrefix(candidate.InterfaceName, "Loopback") {
			t.Fatal("loopback must never be an outbound candidate")
		}
		if candidate.InterfaceName == "Ethernet 2" {
			t.Fatal("down interfaces must never be an outbound candidate")
		}
	}
	if len(candidates) != 5 {
		t.Fatalf("expected 5 candidates, got %d", len(candidates))
	}
}

func TestPhysicalInterfaceWinsOverLowerMetricVirtualAdapter(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, noProbe)

	candidates := collect(FamilyIPv4, Config{}.normalize(), views, routes)
	ranked := rankCandidates(candidates)

	trusted := make([]Candidate, 0, len(ranked))
	for _, candidate := range ranked {
		if !candidate.Untrusted {
			trusted = append(trusted, candidate)
		}
	}
	if len(trusted) == 0 {
		t.Fatal("expected at least one trusted candidate")
	}
	if trusted[0].InterfaceName != "WLAN" {
		t.Fatalf("expected WLAN to win, got %s", trusted[0].InterfaceName)
	}
	if !trusted[0].HasDefaultRoute {
		t.Fatal("WLAN must carry the default route")
	}
	if trusted[0].Gateway.String() != "192.168.1.1" {
		t.Fatalf("unexpected gateway %s", trusted[0].Gateway)
	}
}

func TestHyperVDefaultSwitchLosesDespiteLowestMetric(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, noProbe)

	candidates := collect(FamilyIPv4, Config{}.normalize(), views, routes)
	var hyperV Candidate
	found := false
	for _, candidate := range candidates {
		if candidate.InterfaceName == "vEthernet (Default Switch)" {
			hyperV = candidate
			found = true
		}
	}
	if !found {
		t.Fatal("expected Hyper-V host adapter to be collected as a candidate")
	}
	if !hyperV.Untrusted {
		t.Fatal("Hyper-V host adapter must be flagged untrusted")
	}
	if hyperV.Metric >= 40 {
		t.Fatalf("test fixture must give Hyper-V the lower metric, got %d", hyperV.Metric)
	}

	binding, err := chooseFrom(views, FamilyIPv4, Config{}.normalize(), func(string) {})
	if err != nil {
		t.Fatalf("expected a binding, got %v", err)
	}
	if binding.InterfaceName != "WLAN" {
		t.Fatalf("Hyper-V adapter has the lowest metric but must never win: got %s", binding.InterfaceName)
	}
}

func TestDisableDefaultRoutesFlagDemotesInterface(t *testing.T) {
	views := []InterfaceView{
		{Name: "Ethernet", Index: 3, Up: true, Addresses: []netip.Addr{netip.MustParseAddr("192.168.1.10")}},
	}
	routes := []defaultRoute{
		{interfaceIndex: 3, gateway: netip.MustParseAddr("192.168.1.1"), metric: 25, hasMetric: true, disableDefault: true},
	}
	candidates := collect(FamilyIPv4, Config{}.normalize(), views, routes)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if !candidates[0].Untrusted {
		t.Fatal("interface declaring DisableDefaultRoutes must be flagged untrusted")
	}
	if !candidates[0].DisableDefaultRoute {
		t.Fatal("DisableDefaultRoute flag must be preserved for logging")
	}
}

func TestLowerMetricWinsAmongPhysicalInterfaces(t *testing.T) {
	views := []InterfaceView{
		{Name: "Ethernet", Index: 14, Up: true, Addresses: []netip.Addr{netip.MustParseAddr("10.0.0.8")}},
		{Name: "WLAN", Index: 9, Up: true, Addresses: []netip.Addr{netip.MustParseAddr("192.168.1.42")}},
	}
	routes := []defaultRoute{
		{interfaceIndex: 14, gateway: netip.MustParseAddr("10.0.0.1"), metric: 50, hasMetric: true},
		{interfaceIndex: 9, gateway: netip.MustParseAddr("192.168.1.1"), metric: 35, hasMetric: true},
	}
	ranked := rankCandidates(collect(FamilyIPv4, Config{}.normalize(), views, routes))
	if ranked[0].InterfaceName != "WLAN" {
		t.Fatalf("expected WLAN (metric 35) to outrank Ethernet (metric 50), got %s", ranked[0].InterfaceName)
	}
}

func TestRouteOwnerOutranksInterfaceWithoutDefaultRoute(t *testing.T) {
	views := []InterfaceView{
		{Name: "Ethernet", Index: 5, Up: true, Addresses: []netip.Addr{netip.MustParseAddr("10.0.0.8")}},
		{Name: "WLAN", Index: 9, Up: true, Addresses: []netip.Addr{netip.MustParseAddr("192.168.1.42")}},
	}
	routes := []defaultRoute{
		{interfaceIndex: 9, gateway: netip.MustParseAddr("192.168.1.1"), metric: 900, hasMetric: true},
	}
	ranked := rankCandidates(collect(FamilyIPv4, Config{}.normalize(), views, routes))
	if ranked[0].InterfaceName != "WLAN" {
		t.Fatalf("expected the default-route owner to win, got %s", ranked[0].InterfaceName)
	}
}

func TestFamilyFiltering(t *testing.T) {
	views := testViews()
	v4OnlyRoutes := []defaultRoute{
		{interfaceIndex: 12, gateway: netip.MustParseAddr("172.21.128.1"), metric: 5, hasMetric: true},
		{interfaceIndex: 9, gateway: netip.MustParseAddr("192.168.1.1"), metric: 40, hasMetric: true},
	}

	v6 := collect(FamilyIPv6, Config{}.normalize(), views, v4OnlyRoutes)
	if len(v6) != 1 {
		t.Fatalf("expected only WLAN to offer IPv6, got %d candidates", len(v6))
	}
	if v6[0].InterfaceName != "WLAN" {
		t.Fatalf("expected WLAN for IPv6, got %s", v6[0].InterfaceName)
	}
	if v6[0].HasDefaultRoute {
		t.Fatal("IPv4 routes must not be attributed to an IPv6 selection")
	}
}

func TestLinkLocalAndTunAddressesAreNotUsable(t *testing.T) {
	views := []InterfaceView{
		{Name: "Ethernet", Index: 4, Up: true, Addresses: []netip.Addr{
			netip.MustParseAddr("169.254.13.7"),
			netip.MustParseAddr("198.18.0.7"),
			netip.MustParseAddr("fe80::1"),
		}},
	}
	if candidates := collect(FamilyIPv4, Config{}.normalize(), views, nil); len(candidates) != 0 {
		t.Fatalf("expected no usable IPv4 candidate, got %d", len(candidates))
	}
}

func TestExcludeInterfaceByNameAndByIndex(t *testing.T) {
	views := testViews()
	routes := testRoutes()

	byName := collect(FamilyIPv4, Config{ExcludeInterfaces: []string{"WLAN"}}.normalize(), views, routes)
	for _, candidate := range byName {
		if candidate.InterfaceName == "WLAN" && !candidate.Excluded {
			t.Fatal("WLAN must be excluded by exclude_interface name")
		}
	}

	byGlob := collect(FamilyIPv4, Config{ExcludeInterfaces: []string{"ve*"}}.normalize(), views, routes)
	for _, candidate := range byGlob {
		if strings.HasPrefix(strings.ToLower(candidate.InterfaceName), "ve") && !candidate.Excluded {
			t.Fatalf("%s must be excluded by glob", candidate.InterfaceName)
		}
	}

	byIndex := collect(FamilyIPv4, Config{ExcludeInterfaces: []string{"9"}}.normalize(), views, routes)
	for _, candidate := range byIndex {
		if candidate.InterfaceIndex == 9 && !candidate.Excluded {
			t.Fatal("index 9 must be excluded by exclude_interface index")
		}
	}
}

func TestExcludeAddressRemovesCandidate(t *testing.T) {
	views := testViews()
	routes := testRoutes()

	candidates := collect(FamilyIPv4, Config{ExcludeAddresses: []string{"192.168.1.0/24"}}.normalize(), views, routes)
	for _, candidate := range candidates {
		if candidate.InterfaceName == "WLAN" {
			if !candidate.Excluded {
				t.Fatal("WLAN address falls inside route_exclude_address and must be excluded")
			}
			if candidate.ExcludedBy != "route_exclude_address=192.168.1.0/24" {
				t.Fatalf("unexpected exclusion reason %q", candidate.ExcludedBy)
			}
		}
	}

	single := collect(FamilyIPv4, Config{ExcludeAddresses: []string{"10.0.0.8"}}.normalize(), views, routes)
	for _, candidate := range single {
		if candidate.InterfaceName == "Ethernet" && !candidate.Excluded {
			t.Fatal("bare address exclusion must apply")
		}
	}
}

func TestIncludeInterfaceForcesTrust(t *testing.T) {
	views := testViews()
	routes := testRoutes()

	candidates := collect(FamilyIPv4, Config{IncludeInterfaces: []string{"VEthernet (Default Switch)"}}.normalize(), views, routes)
	for _, candidate := range candidates {
		if candidate.InterfaceName == "vEthernet (Default Switch)" {
			if candidate.Untrusted {
				t.Fatal("include_interface must force the adapter to be trusted")
			}
			if !candidate.ForcedInclude {
				t.Fatal("forced include must be recorded")
			}
		}
	}
}

func TestChooseIgnoresExclusionsWhenNothingRemains(t *testing.T) {
	views := []InterfaceView{
		{Name: "WLAN", Index: 9, Up: true, Addresses: []netip.Addr{netip.MustParseAddr("192.168.1.42")}},
	}
	routes := []defaultRoute{
		{interfaceIndex: 9, gateway: netip.MustParseAddr("192.168.1.1"), metric: 35, hasMetric: true},
	}
	stubResolver(t, routes, noProbe)

	var logs []string
	binding, err := chooseFrom(views, FamilyIPv4, Config{ExcludeInterfaces: []string{"wlan"}}.normalize(), collectLogs(&logs))
	if err != nil {
		t.Fatalf("expected fallback binding, got error %v", err)
	}
	if binding.InterfaceName != "WLAN" {
		t.Fatalf("expected WLAN fallback, got %s", binding.InterfaceName)
	}
	if !containsLog(logs, "ignoring exclusions") {
		t.Fatalf("expected exclusion-relaxation log, got %v", logs)
	}
}

func TestChooseFallsBackToVirtualAdapterWhenNoPhysicalExists(t *testing.T) {
	views := []InterfaceView{
		{Name: "vEthernet (Default Switch)", Index: 12, Up: true, Addresses: []netip.Addr{netip.MustParseAddr("172.21.128.1")}},
	}
	routes := []defaultRoute{
		{interfaceIndex: 12, gateway: netip.MustParseAddr("172.21.128.1"), metric: 5, hasMetric: true},
	}
	stubResolver(t, routes, noProbe)

	binding, err := chooseFrom(views, FamilyIPv4, Config{}.normalize(), func(string) {})
	if err != nil {
		t.Fatalf("expected virtual fallback, got error %v", err)
	}
	if binding.Trusted {
		t.Fatal("fallback binding must be reported as untrusted")
	}
	if binding.Mode != "route-table-virtual-fallback" {
		t.Fatalf("unexpected mode %q", binding.Mode)
	}
}

func TestChooseSkipsProbeFailuresInRankOrder(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, func(candidate Candidate) error {
		if candidate.InterfaceName == "WLAN" {
			return &netUnreachableError{}
		}
		return nil
	})

	binding, err := chooseFrom(views, FamilyIPv4, Config{}.normalize(), func(string) {})
	if err != nil {
		t.Fatalf("expected fallback candidate, got error %v", err)
	}
	if binding.InterfaceName != "Ethernet" {
		t.Fatalf("expected Ethernet after WLAN probe failure, got %s", binding.InterfaceName)
	}
}

func TestChooseReturnsLastResortWhenAllProbesFail(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, func(Candidate) error { return &netUnreachableError{} })

	binding, err := chooseFrom(views, FamilyIPv4, Config{}.normalize(), func(string) {})
	if err != nil {
		t.Fatalf("expected last-resort binding, got error %v", err)
	}
	if binding.InterfaceName != "WLAN" {
		t.Fatalf("expected top-ranked WLAN, got %s", binding.InterfaceName)
	}
	if !strings.HasSuffix(binding.Mode, "probe-failed") {
		t.Fatalf("unexpected mode %q", binding.Mode)
	}
}

func TestChooseWithoutAnyCandidate(t *testing.T) {
	stubResolver(t, nil, noProbe)
	if _, err := chooseFrom(nil, FamilyIPv4, Config{}.normalize(), func(string) {}); err != ErrNoUsableInterface {
		t.Fatalf("expected ErrNoUsableInterface, got %v", err)
	}
}

func TestBindingLocalAddrHonoursZone(t *testing.T) {
	binding := Binding{Address: netip.MustParseAddr("fe80::1").WithZone("12")}
	if binding.LocalTCPAddr().Zone != "12" {
		t.Fatalf("expected zone 12, got %q", binding.LocalTCPAddr().Zone)
	}
	if binding.LocalUDPAddr().Zone != "12" {
		t.Fatalf("expected zone 12, got %q", binding.LocalUDPAddr().Zone)
	}
	plain := Binding{Address: netip.MustParseAddr("192.168.1.42")}
	if plain.LocalTCPAddr().String() != "192.168.1.42:0" {
		t.Fatalf("unexpected local addr %s", plain.LocalTCPAddr())
	}
}

func TestDescribeContainsDiagnosticFields(t *testing.T) {
	binding := Binding{
		Address:        netip.MustParseAddr("192.168.1.42"),
		InterfaceName:  "WLAN",
		InterfaceIndex: 9,
		Gateway:        netip.MustParseAddr("192.168.1.1"),
		Metric:         35,
		Trusted:        true,
		DefaultRoute:   true,
		Mode:           "route-table",
	}
	text := binding.Describe()
	for _, fragment := range []string{"interface=WLAN", "index=9", "address=192.168.1.42", "gateway=192.168.1.1", "metric=35", "physical=true", "default_route=true"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("Describe() missing %q in %q", fragment, text)
		}
	}
}

func TestConfigNormalizationIsStable(t *testing.T) {
	cfg := Config{
		ExcludeInterfaces: []string{" WLAN ", "wlan", "VEthernet*"},
		ExcludeAddresses:  []string{"10.0.0.0/8", "", "192.168.1.1"},
	}.normalize()

	if len(cfg.excludeInterfaces) != 2 {
		t.Fatalf("expected deduplicated patterns, got %v", cfg.excludeInterfaces)
	}
	if cfg.excludeInterfaces[0] != "vethernet*" || cfg.excludeInterfaces[1] != "wlan" {
		t.Fatalf("unexpected patterns %v", cfg.excludeInterfaces)
	}
	if len(cfg.excludeAddresses) != 2 {
		t.Fatalf("expected 2 prefixes, got %v", cfg.excludeAddresses)
	}
	if cfg.excludeAddresses[1].String() != "192.168.1.1/32" {
		t.Fatalf("unexpected prefix %v", cfg.excludeAddresses[1])
	}
	if cfg.key == "" {
		t.Fatal("cache key must not be empty")
	}
}

func TestMatchesPatternSupportsIndexAndGlob(t *testing.T) {
	if _, ok := matchesPattern([]string{"9"}, "WLAN", 9); !ok {
		t.Fatal("numeric pattern must match interface index")
	}
	if _, ok := matchesPattern([]string{"wlan*"}, "Ethernet", 4); ok {
		t.Fatal("glob wlan* must not match Ethernet")
	}
	if _, ok := matchesPattern([]string{"eth*"}, "eth0", 4); !ok {
		t.Fatal("glob eth* must match eth0")
	}
	if _, ok := matchesPattern([]string{"enp*"}, "enp3s0", 4); !ok {
		t.Fatal("glob enp* must match enp3s0")
	}
}

func TestProbeCandidateWithoutGatewayIsAccepted(t *testing.T) {
	if err := defaultProbe(Candidate{Address: netip.MustParseAddr("192.168.1.42")}); err != nil {
		t.Fatalf("gateway-less candidate must be accepted, got %v", err)
	}
}

func TestForceInterfaceRestrictsSelectionToThatInterface(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, noProbe)

	var logs []string
	binding, err := chooseFrom(views, FamilyIPv4, Config{ForceInterface: "Ethernet"}.normalize(), collectLogs(&logs))
	if err != nil {
		t.Fatalf("expected forced binding, got %v", err)
	}
	if binding.InterfaceName != "Ethernet" {
		t.Fatalf("expected Ethernet, got %s", binding.InterfaceName)
	}
	if binding.Mode != "user-selected" {
		t.Fatalf("unexpected mode %q", binding.Mode)
	}
	if !containsLog(logs, "user-selected outbound binding") {
		t.Fatalf("expected user-selected log, got %v", logs)
	}
}

func TestForceInterfaceAcceptsIndexAndVirtualAdapter(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, noProbe)

	byIndex, err := chooseFrom(views, FamilyIPv4, Config{ForceInterface: "14"}.normalize(), func(string) {})
	if err != nil {
		t.Fatalf("expected index-based binding, got %v", err)
	}
	if byIndex.InterfaceName != "Ethernet" {
		t.Fatalf("expected Ethernet for index 14, got %s", byIndex.InterfaceName)
	}

	byVirtual, err := chooseFrom(views, FamilyIPv4, Config{ForceInterface: "vEthernet (Default Switch)"}.normalize(), func(string) {})
	if err != nil {
		t.Fatalf("expected virtual binding, got %v", err)
	}
	if byVirtual.InterfaceName != "vEthernet (Default Switch)" {
		t.Fatalf("expected the selected virtual adapter, got %s", byVirtual.InterfaceName)
	}
	if !byVirtual.Trusted {
		t.Fatal("an explicitly selected adapter must be reported as trusted")
	}
}

func TestForceInterfaceFallsBackToSmartWhenUnusable(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, noProbe)

	var logs []string
	binding, err := chooseFrom(views, FamilyIPv4, Config{ForceInterface: "no-such-adapter"}.normalize(), collectLogs(&logs))
	if err != nil {
		t.Fatalf("expected smart fallback, got %v", err)
	}
	if binding.InterfaceName != "WLAN" {
		t.Fatalf("expected smart selection WLAN, got %s", binding.InterfaceName)
	}
	if !containsLog(logs, "falling back to smart selection") {
		t.Fatalf("expected fallback log, got %v", logs)
	}
}

func TestForceInterfaceFallsBackWhenProbeRejects(t *testing.T) {
	views := testViews()
	routes := testRoutes()
	stubResolver(t, routes, func(candidate Candidate) error {
		if candidate.InterfaceName == "Ethernet" {
			return &netUnreachableError{}
		}
		return nil
	})

	var logs []string
	binding, err := chooseFrom(views, FamilyIPv4, Config{ForceInterface: "Ethernet"}.normalize(), collectLogs(&logs))
	if err != nil {
		t.Fatalf("expected smart fallback, got %v", err)
	}
	if binding.InterfaceName == "Ethernet" {
		t.Fatal("a forced interface that fails the probe must not be used")
	}
	if !containsLog(logs, "falling back to smart selection") {
		t.Fatalf("expected fallback log, got %v", logs)
	}
}

func TestListExposesInterfacesForUI(t *testing.T) {
	descriptors := List(Config{})
	if len(descriptors) == 0 {
		t.Skip("no non-loopback interface on this host")
	}
	for _, descriptor := range descriptors {
		if descriptor.Name == "" {
			t.Fatal("descriptor must carry a name")
		}
		if descriptor.Index <= 0 {
			t.Fatalf("descriptor %s must carry a positive index", descriptor.Name)
		}
		if strings.Contains(descriptor.Name, "Loopback Pseudo") {
			t.Fatal("loopback must not be listed")
		}
	}
	for i := 1; i < len(descriptors); i++ {
		if descriptors[i-1].Virtual && !descriptors[i].Virtual {
			t.Fatal("physical interfaces must be listed before virtual adapters")
		}
	}
}

type netUnreachableError struct{}

func (e *netUnreachableError) Error() string { return "unreachable network" }

func collectLogs(target *[]string) func(string) {
	return func(message string) { *target = append(*target, message) }
}

func containsLog(logs []string, fragment string) bool {
	for _, line := range logs {
		if strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}

func TestSelectCacheReusesResult(t *testing.T) {
	original := lookupRoutes
	originalProbe := probeCandidateRoute
	calls := 0
	lookupRoutes = func(int) []defaultRoute {
		calls++
		return testRoutes()
	}
	probeCandidateRoute = noProbe
	t.Cleanup(func() {
		lookupRoutes = original
		probeCandidateRoute = originalProbe
		InvalidateCache()
	})
	InvalidateCache()

	first, err := Select(FamilyIPv4, Config{ExcludeInterfaces: []string{"cached-test-marker"}}, func(string) {})
	second, err2 := Select(FamilyIPv4, Config{ExcludeInterfaces: []string{"cached-test-marker"}}, func(string) {})
	if err != nil || err2 != nil {
		t.Skipf("no candidate on this host: %v / %v", err, err2)
	}
	if first == nil || second == nil || first.InterfaceName != second.InterfaceName {
		t.Fatal("cached selection must return the same interface")
	}
	if calls != 1 {
		t.Fatalf("expected exactly one route-table lookup, got %d", calls)
	}
}
