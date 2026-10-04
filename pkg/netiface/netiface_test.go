package netiface

import (
	"net/netip"
	"testing"
)

// TestCollectPrefersDefaultRouteOverEnumerationOrder is the regression test for
// the original defect: net.Interfaces() returns adapters by index, so a
// virtual adapter with a lower index (VMware VMnet, Hyper-V, WSL) was picked
// ahead of the adapter that actually owned the default route.
func TestCollectPrefersDefaultRouteOverEnumerationOrder(t *testing.T) {
	views := []interfaceView{
		{Index: 13, Name: "VMware Network Adapter VMnet8", Up: true, Addresses: addrs("192.168.230.1")},
		{Index: 20, Name: "VMware Network Adapter VMnet1", Up: true, Addresses: addrs("192.168.153.1")},
		{Index: 27, Name: "WLAN", Up: true, Addresses: addrs("192.168.1.6")},
	}
	routes := map[int]uint32{27: 35}

	binding, ok := choose(collect(FamilyIPv4, Config{}, views, routes), netip.Addr{})
	if !ok {
		t.Fatal("expected a binding")
	}
	if binding.InterfaceName != "WLAN" || binding.InterfaceIndex != 27 {
		t.Fatalf("selected %s (idx %d), want WLAN (idx 27)", binding.InterfaceName, binding.InterfaceIndex)
	}
	if binding.Address.String() != "192.168.1.6" {
		t.Fatalf("selected address %s, want 192.168.1.6", binding.Address)
	}
	if !binding.DefaultRoute {
		t.Fatal("expected DefaultRoute to be reported")
	}
}

// TestCollectRejectsLinkLocalIPv6 pins the second half of the original defect:
// the first enumerated IPv6 address on a host is usually fe80::, and binding
// it produces an unroutable socket.
func TestCollectRejectsLinkLocalIPv6(t *testing.T) {
	views := []interfaceView{
		{Index: 20, Name: "VMware Network Adapter VMnet1", Up: true, Addresses: addrs("fe80::1")},
		{Index: 27, Name: "WLAN", Up: true, Addresses: addrs("2409:8a10:1422:c130::3")},
	}
	routes := map[int]uint32{27: 291}

	binding, ok := choose(collect(FamilyIPv6, Config{}, views, routes), netip.Addr{})
	if !ok {
		t.Fatal("expected a binding")
	}
	if binding.Address.IsLinkLocalUnicast() {
		t.Fatalf("selected link-local address %s, want a global address", binding.Address)
	}
	if binding.Address.String() != "2409:8a10:1422:c130::3" {
		t.Fatalf("selected address %s, want the WLAN global address", binding.Address)
	}
}

// TestKernelProbeWins verifies the kernel routing decision outranks the
// default-route table, which matters when the two disagree (for example while
// a tunnel is capturing routes).
func TestKernelProbeWins(t *testing.T) {
	views := []interfaceView{
		{Index: 10, Name: "Ethernet", Up: true, Addresses: addrs("10.0.0.5")},
		{Index: 27, Name: "WLAN", Up: true, Addresses: addrs("192.168.1.6")},
	}
	routes := map[int]uint32{27: 35}
	probe := netip.MustParseAddr("10.0.0.5")

	binding, ok := choose(collect(FamilyIPv4, Config{}, views, routes), probe)
	if !ok {
		t.Fatal("expected a binding")
	}
	if binding.InterfaceName != "Ethernet" {
		t.Fatalf("selected %s, want Ethernet (kernel probe match)", binding.InterfaceName)
	}
	if binding.Reason != "kernel_route_probe" {
		t.Fatalf("reason = %q, want kernel_route_probe", binding.Reason)
	}
}

// TestVirtualAdapterDemotedButUsable makes sure a host whose only uplink is a
// virtual adapter still gets a binding instead of failing outright.
func TestVirtualAdapterDemotedButUsable(t *testing.T) {
	views := []interfaceView{
		{Index: 8, Name: "vEthernet (WSL)", Up: true, Addresses: addrs("172.28.240.1")},
	}

	binding, ok := choose(collect(FamilyIPv4, Config{}, views, map[int]uint32{}), netip.Addr{})
	if !ok {
		t.Fatal("expected a fallback binding on the virtual adapter")
	}
	if !binding.Virtual {
		t.Fatal("expected Virtual to be reported")
	}
	if binding.Address.String() != "172.28.240.1" {
		t.Fatalf("selected address %s, want 172.28.240.1", binding.Address)
	}
}

// TestOwnTunnelAndLoopbackExcluded ensures the tunnel's own adapter is never
// selected as the outbound path, which would loop traffic back into the proxy.
func TestOwnTunnelAndLoopbackExcluded(t *testing.T) {
	views := []interfaceView{
		{Index: 1, Name: "Loopback Pseudo-Interface 1", Up: true, Loopback: true, Addresses: addrs("127.0.0.1", "::1")},
		{Index: 33, Name: "SniShaper", Up: true, Addresses: addrs("198.18.0.1", "fd65:198:18::1")},
		{Index: 27, Name: "WLAN", Up: true, Addresses: addrs("192.168.1.6")},
	}

	binding, ok := choose(collect(FamilyIPv4, Config{}, views, map[int]uint32{27: 35}), netip.Addr{})
	if !ok {
		t.Fatal("expected a binding")
	}
	if binding.InterfaceName != "WLAN" {
		t.Fatalf("selected %s, want WLAN", binding.InterfaceName)
	}
}

// TestTunPrefixAddressesRejected keeps the tunnel's own address space from
// being used as a source address.
func TestTunPrefixAddressesRejected(t *testing.T) {
	for _, raw := range []string{"198.18.0.1", "198.19.255.254", "fd65:198:18::1"} {
		addr := netip.MustParseAddr(raw)
		if usableAddress(addr, FamilyIPv4, true) || usableAddress(addr, FamilyIPv6, true) {
			t.Errorf("usableAddress(%s) = true, want false", raw)
		}
	}
}

// TestExcludeAndPreferNames covers the user-facing escape hatches.
func TestExcludeAndForceInterface(t *testing.T) {
	views := []interfaceView{
		{Index: 13, Name: "VMware Network Adapter VMnet8", Up: true, Addresses: addrs("192.168.230.1")},
		{Index: 27, Name: "WLAN", Up: true, Addresses: addrs("192.168.1.6")},
	}

	cfg := Config{ExcludeInterfaces: []string{"VMware*"}}
	binding, ok := choose(collect(FamilyIPv4, cfg, views, map[int]uint32{13: 1, 27: 35}), netip.Addr{})
	if !ok {
		t.Fatal("expected a binding")
	}
	if binding.InterfaceName != "WLAN" {
		t.Fatalf("selected %s, want WLAN after exclusion", binding.InterfaceName)
	}
}

// TestIsOwnTunnelMatchesWholeNames guards against substring matching, which
// would misclassify unrelated adapters.
func TestIsOwnTunnelMatchesWholeNames(t *testing.T) {
	for _, name := range []string{"SniShaper", "snishaper", "singtun", "sing-tun", "SniShaper2"} {
		if !IsOwnTunnel(name) {
			t.Errorf("IsOwnTunnel(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"WLAN", "Ethernet", "Tundra", "SetupVPN", "tun"} {
		if IsOwnTunnel(name) {
			t.Errorf("IsOwnTunnel(%q) = true, want false", name)
		}
	}
}

func TestFamilyOf(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"1.1.1.1", FamilyIPv4},
		{"1.1.1.1:443", FamilyIPv4},
		{"2606:4700::1111", FamilyIPv6},
		{"[2606:4700::1111]:443", FamilyIPv6},
		{"example.com:443", FamilyIPv4},
		{"example.com", FamilyIPv4},
	}
	for _, tc := range cases {
		if got := FamilyOf(tc.in); got != tc.want {
			t.Errorf("FamilyOf(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// TestBuiltinBlacklistExcludesVirtualNICs pins the built-in blacklist: virtual
// adapters that own an address must never win auto-selection.
func TestBuiltinBlacklistExcludesVirtualNICs(t *testing.T) {
	blacklisted := []string{
		"wg0", "tun2proxy0", "utun3", "docker0", "veth1234", "virbr0",
		"vmnet1", "vboxnet0", "br0", "lo", "Meta", "sing-box", "cni0",
		"flannel.1", "cali0", "weave", "dummy0", "ifb0", "ip6tnl0",
	}
	for _, name := range blacklisted {
		if !isDefaultExcluded(name) {
			t.Errorf("isDefaultExcluded(%q) = false, want true", name)
		}
	}

	// 真实网卡不能被黑名单误伤。
	for _, name := range []string{"WLAN", "Ethernet", "以太网", "WLAN 2", "tailscale0"} {
		if name == "tailscale0" {
			continue
		}
		if isDefaultExcluded(name) {
			t.Errorf("isDefaultExcluded(%q) = true, want false", name)
		}
	}
}

// TestBlacklistYieldsToProbeWhenNoRouteExists makes sure a machine whose only
// uplink is itself a tunnel device still gets a usable binding instead of
// failing outright.
func TestBlacklistYieldsWhenNoRouteExists(t *testing.T) {
	views := []interfaceView{
		{Index: 5, Name: "wg0", Up: true, Addresses: addrs("10.8.0.2")},
	}

	binding, ok := choose(collect(FamilyIPv4, Config{}, views, map[int]uint32{}), netip.Addr{})
	if !ok {
		t.Fatal("expected a fallback binding on the tunnelled uplink")
	}
	if binding.InterfaceName != "wg0" {
		t.Fatalf("selected %s, want wg0", binding.InterfaceName)
	}
}

// TestExplicitPreferenceOverridesBlacklist verifies a user-chosen interface is
// honoured even when the built-in blacklist would drop it.
func TestExplicitPreferenceOverridesBlacklist(t *testing.T) {
	views := []interfaceView{
		{Index: 5, Name: "wg0", Up: true, Addresses: addrs("10.8.0.2")},
		{Index: 27, Name: "WLAN", Up: true, Addresses: addrs("192.168.1.6")},
	}
	cfg := Config{ForceInterface: "wg0"}

	binding, ok := choose(collect(FamilyIPv4, cfg, views, map[int]uint32{27: 35}), netip.Addr{})
	if !ok {
		t.Fatal("expected a binding")
	}
	if binding.InterfaceName != "wg0" {
		t.Fatalf("selected %s, want the explicitly preferred wg0", binding.InterfaceName)
	}
}

func addrs(values ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(values))
	for _, value := range values {
		out = append(out, netip.MustParseAddr(value))
	}
	return out
}
