// Package netiface selects the physical NIC that owns the system default
// route and binds outbound sockets to it.
//
// Picking a NIC by enumerating net.Interfaces() is wrong: enumeration order is
// by interface index, not by routing preference, so a virtual adapter that
// happens to own a lower index (VMware VMnet, Hyper-V vEthernet, WSL, a
// leftover TAP) wins over the adapter that actually carries traffic. On a
// dual-stack host the first enumerated IPv6 address is very often a link-local
// fe80:: address, which is unroutable off-link.
//
// The rule implemented here is the same one sing-box and mihomo use: ask the
// kernel which interface owns the default route, and only fall back to
// enumeration heuristics when the kernel gives no answer.
package netiface

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

// Address families accepted by Select.
const (
	FamilyIPv4 = iota
	FamilyIPv6
)

// FamilyIPv4Of reports the family of addr, defaulting to IPv4 when addr is not
// an IP literal.
func FamilyOf(addr string) int {
	parsed, err := netip.ParseAddr(addr)
	if err != nil {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			if parsed, err = netip.ParseAddr(host); err != nil {
				return FamilyIPv4
			}
		} else {
			return FamilyIPv4
		}
	}
	if parsed.Is4() {
		return FamilyIPv4
	}
	return FamilyIPv6
}

// tunPrefixes are the address ranges SniShaper assigns to its own TUN device.
// An address inside them is never a valid outbound source.
var tunPrefixes = []netip.Prefix{
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("fd65:198:18::/64"),
}

// tunNames are the interface names SniShaper gives its own devices. Matching
// is exact-ish: a name that merely contains one of these as a substring of a
// longer word (for example "Tundra" or "SetupVPN") is not treated as the TUN.
var tunNames = []string{
	"snishaper",
	"sni-shaper",
	"singtun",
	"sing-tun",
}

// virtualNameMarkers identify adapters that are not real Internet uplinks.
// They are only demoted, never hard-excluded: a host whose only uplink is a
// virtual adapter (a cloud VM, a VPN-only machine) must still work.
var virtualNameMarkers = []string{
	"vmware",
	"virtualbox",
	"vbox",
	"hyper-v",
	"vethernet",
	"default switch",
	"docker",
	"wsl",
	"bridge",
	"tap",
	"tun",
	"loopback",
	"pseudo-interface",
	"bluetooth",
	"teredo",
	"isatap",
	"npcap",
}

// DefaultExcludedNames is the built-in blacklist of interfaces that must never
// carry SniShaper outbound traffic. These are container bridges, virtual
// machine host adapters, tunnel devices and other software-only interfaces
// that have no presence on any physical medium; sending traffic through one
// either blackholes it or feeds it back into the tunnel.
//
// Unlike virtualNameMarkers, which only demotes, entries here are excluded
// outright during auto-selection. Users can extend the list through
// TUNConfig.OutboundInterfaceExclude.
var DefaultExcludedNames = []string{
	// Tunnel / VPN / proxy devices
	"tun2proxy*", "utun*", "tun*", "tap*", "wg*", "ppp*", "ip6tnl*", "ip6gre*",
	"ifb*", "teql*", "gre*", "gretap*", "erspan*", "sit*", "ipip*",
	// Container / VM bridges
	"docker*", "veth*", "virbr*", "vmnet*", "vboxnet*", "vnic*", "br-*", "br0",
	"vmbr*", "vbox*", "vnic*", "veth*", "cni*", "flannel*", "cali*", "weave*",
	"lxc*", "lxd*", "podman*", "nerdctl*",
	// Cluster / overlay networks
	"kube-ipvs*", "cilium*", "flannel*", "cali*", "weave*", "kube*",
	// Carrier / Tailscale / ZeroTier style overlays
	"tailscale*", "zt*", "wg*", "ham*", "ca*",
	// Misc virtual
	"rmnet*", "wwan*", "dummy*", "ifb*", "lo", "lo*",
	// Meta / clash style TUN devices
	"Meta", "meta", "mihomo", "Mihomo", "clash", "Clash", "sing-box*", "SingTun*",
}

// isDefaultExcluded reports whether name matches the built-in blacklist.
// Entries ending in "*" are prefix globs; everything else is matched
// case-insensitively as a whole name so unrelated adapters are not caught.
func isDefaultExcluded(name string) bool {
	if IsOwnTunnel(name) {
		return true
	}
	if _, ok := matchesPattern(DefaultExcludedNames, name, 0); ok {
		return true
	}
	lowered := strings.ToLower(name)
	for _, entry := range DefaultExcludedNames {
		if !strings.HasSuffix(entry, "*") && strings.EqualFold(entry, lowered) {
			return true
		}
	}
	return false
}

// Config tunes interface selection. The zero value is the production default.
type Config struct {
	// ForceInterface pins selection to the named interface. Empty keeps
	// automatic selection. An entry ending in "*" is a prefix glob.
	ForceInterface string

	// ExcludeInterfaces drops interfaces whose name matches one of these
	// entries. Comparison is case-insensitive; "name*" is a prefix glob.
	ExcludeInterfaces []string

	// ExcludeAddresses drops interfaces carrying an address inside one of these
	// CIDR blocks or literal addresses.
	ExcludeAddresses []string
}

// Binding is a resolved physical NIC to send outbound traffic through.
type Binding struct {
	// InterfaceIndex is the OS interface index, used for IP_UNICAST_IF and
	// SO_BINDTOIFINDEX.
	InterfaceIndex int
	// InterfaceName is the OS interface name.
	InterfaceName string
	// Address is a usable source address on that interface for the requested
	// family. It is invalid when the interface is link-local only.
	Address netip.Addr
	// DefaultRoute reports whether the interface owns a default route.
	DefaultRoute bool
	// Metric is the interface plus route metric; lower is better.
	Metric uint32
	// Virtual reports whether the adapter looks virtual (demoted, not banned).
	Virtual bool
	// Reason explains the selection for logs and tests.
	Reason string
}

// Describe renders the binding for diagnostics.
func (b Binding) Describe() string {
	addr := "none"
	if b.Address.IsValid() {
		addr = b.Address.String()
	}
	return fmt.Sprintf(
		"interface=%s index=%d address=%s default_route=%t metric=%d virtual=%t source=%s",
		b.InterfaceName, b.InterfaceIndex, addr, b.DefaultRoute, b.Metric, b.Virtual, b.Reason,
	)
}

// LocalTCPAddr returns a TCP local address for the binding, or nil when no
// source address is available.
func (b Binding) LocalTCPAddr() *net.TCPAddr {
	if !b.Address.IsValid() {
		return nil
	}
	addr := &net.TCPAddr{IP: net.IP(b.Address.AsSlice())}
	if b.Address.Is6() && b.Address.IsLinkLocalUnicast() && b.Address.Zone() != "" {
		addr.Zone = b.Address.Zone()
	}
	return addr
}

// LocalUDPAddr returns a UDP local address for the binding, or nil when no
// source address is available.
func (b Binding) LocalUDPAddr() *net.UDPAddr {
	if !b.Address.IsValid() {
		return nil
	}
	addr := &net.UDPAddr{IP: net.IP(b.Address.AsSlice())}
	if b.Address.Is6() && b.Address.IsLinkLocalUnicast() && b.Address.Zone() != "" {
		addr.Zone = b.Address.Zone()
	}
	return addr
}

// interfaceView is a snapshot of one adapter, decoupled from net.Interface so
// selection can be unit tested without touching the host.
type interfaceView struct {
	Index     int
	Name      string
	Up        bool
	Loopback  bool
	Addresses []netip.Addr
}

// candidate is a scored interface.
type candidate struct {
	view         interfaceView
	address      netip.Addr
	hasRoute     bool
	metric       uint32
	virtual      bool
	excluded     bool
	preferred    bool
	excludedBy   string
	usableSource bool
}

// IsOwnTunnel reports whether name belongs to a SniShaper TUN device.
func IsOwnTunnel(name string) bool {
	lowered := strings.ToLower(strings.TrimSpace(name))
	for _, candidate := range tunNames {
		if lowered == candidate {
			return true
		}
	}
	// Windows adapters created by sing-tun may carry a numeric suffix.
	for _, candidate := range tunNames {
		if rest, ok := strings.CutPrefix(lowered, candidate); ok && isAllDigits(rest) {
			return true
		}
	}
	return false
}

// IsVirtualAdapter reports whether name looks like a virtual or tunnel
// adapter rather than a physical uplink.
func IsVirtualAdapter(name string) bool {
	lowered := strings.ToLower(name)
	for _, marker := range virtualNameMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// inTunPrefix reports whether addr belongs to SniShaper's own TUN addressing.
func inTunPrefix(addr netip.Addr) bool {
	for _, prefix := range tunPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// usableAddress reports whether addr can serve as an outbound source for the
// given family. Link-local addresses are rejected: they are unroutable off-link
// and binding one produces "can't assign requested address" at connect time.
func usableAddress(addr netip.Addr, family int, withZone bool) bool {
	if !addr.IsValid() {
		return false
	}
	if family == FamilyIPv4 && !addr.Is4() {
		return false
	}
	if family == FamilyIPv6 && !addr.Is6() {
		return false
	}
	if addr.IsLoopback() || addr.IsUnspecified() || addr.IsMulticast() ||
		addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() {
		return false
	}
	if addr.IsLinkLocalUnicast() {
		// fe80:: is only usable with an explicit zone, and even then only for
		// on-link destinations. Never treat it as a global default source.
		return withZone && family == FamilyIPv6 && addr.Zone() != ""
	}
	return !inTunPrefix(addr)
}

// matchesPattern reports whether name matches an include/exclude entry. An
// entry ending in "*" is a prefix match, everything else is exact.
func matchesPattern(patterns []string, name string, index int) (string, bool) {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "*") {
			if strings.HasPrefix(name, strings.TrimSuffix(pattern, "*")) {
				return pattern, true
			}
			continue
		}
		if strings.EqualFold(name, pattern) {
			return pattern, true
		}
		if isAllDigits(pattern) && index == atoiOrZero(pattern) {
			return pattern, true
		}
	}
	return "", false
}

func atoiOrZero(s string) int {
	value := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		value = value*10 + int(r-'0')
	}
	return value
}

// selectAddress picks the best source address on a view for the family.
// Global addresses beat link-local ones.
func selectAddress(view interfaceView, family int) (netip.Addr, bool) {
	var fallback netip.Addr
	for _, addr := range view.Addresses {
		if !usableAddress(addr, family, true) {
			continue
		}
		if !addr.IsLinkLocalUnicast() {
			return addr, true
		}
		if !fallback.IsValid() {
			fallback = addr
		}
	}
	return fallback, fallback.IsValid()
}

// collect turns interface snapshots plus default routes into scored
// candidates. Default-route ownership is the dominant signal.
func collect(family int, cfg Config, views []interfaceView, routes map[int]uint32) []candidate {
	candidates := make([]candidate, 0, len(views))
	for _, view := range views {
		item := candidate{view: view}

		if !view.Up || view.Loopback || IsOwnTunnel(view.Name) {
			continue
		}
		if pattern, ok := matchesPattern(cfg.ExcludeInterfaces, view.Name, view.Index); ok {
			item.excluded = true
			item.excludedBy = "exclude=" + pattern
		}
		// 用户显式指定优先级最高，可以覆盖黑名单与降权。
		if isForced(cfg.ForceInterface, view.Name, view.Index) {
			item.preferred = true
		} else if isDefaultExcluded(view.Name) {
			// 内置黑名单：容器桥、虚拟机宿主网卡、隧道设备等纯软件接口。
			// 这些网卡没有物理介质，走它们要么丢包要么把流量绕回隧道。
			item.excluded = true
			item.excludedBy = "builtin_blacklist"
		}

		item.virtual = IsVirtualAdapter(view.Name)

		if metric, ok := routes[view.Index]; ok {
			item.hasRoute = true
			item.metric = metric
		}
		if addr, ok := selectAddress(view, family); ok {
			item.address = addr
			item.usableSource = true
			if prefix, hit := matchesAddress(cfg.ExcludeAddresses, addr); hit {
				item.excluded = true
				item.excludedBy = "exclude_address=" + prefix.String()
			}
		}
		candidates = append(candidates, item)
	}
	return candidates
}

// isForced reports whether name is the interface the user pinned in config.
func isForced(force string, name string, index int) bool {
	force = strings.TrimSpace(force)
	if force == "" {
		return false
	}
	_, ok := matchesPattern([]string{force}, name, index)
	return ok
}

// matchesAddress reports whether addr falls inside any configured address
// exclusion, accepting both CIDR blocks and bare addresses.
func matchesAddress(values []string, addr netip.Addr) (netip.Prefix, bool) {
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "/") {
			prefix, err := netip.ParsePrefix(trimmed)
			if err != nil {
				continue
			}
			if prefix.Contains(addr) {
				return prefix, true
			}
			continue
		}
		if literal, err := netip.ParseAddr(trimmed); err == nil && literal == addr {
			return netip.PrefixFrom(literal, literal.BitLen()), true
		}
	}
	return netip.Prefix{}, false
}

// rank orders candidates best-first. Preference order:
//  1. explicitly preferred by config (a user choice outranks any heuristic)
//  2. the address the kernel routing probe confirmed
//  3. owns a default route
//  4. real uplink rather than virtual adapter
//  5. lower route metric
//  6. lower interface index (stable, matches OS preference)
func rank(candidates []candidate, probe netip.Addr) []candidate {
	ordered := make([]candidate, 0, len(candidates))
	for _, item := range candidates {
		if !item.excluded {
			ordered = append(ordered, item)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.preferred != right.preferred {
			return left.preferred
		}
		leftProbe := probe.IsValid() && left.address == probe
		rightProbe := probe.IsValid() && right.address == probe
		if leftProbe != rightProbe {
			return leftProbe
		}
		if left.hasRoute != right.hasRoute {
			return left.hasRoute
		}
		if left.virtual != right.virtual {
			return !left.virtual
		}
		if left.metric != right.metric {
			return left.metric < right.metric
		}
		if left.view.Index != right.view.Index {
			return left.view.Index < right.view.Index
		}
		return left.address.Less(right.address)
	})
	return ordered
}

// probeTargets are well-known public addresses used to ask the kernel which
// source address it would pick for off-link traffic. Connecting a UDP socket
// sends no packets; it only runs the routing decision.
var (
	probeTargetIPv4 = &net.UDPAddr{IP: net.IPv4(8, 8, 8, 8), Port: 53}
	probeTargetIPv6 = &net.UDPAddr{IP: net.ParseIP("2001:4860:4860::8888"), Port: 53}
)

// probeSourceAddr returns the source address the kernel itself selects for
// outbound traffic of the given family.
//
// This is the most reliable signal available: instead of re-implementing the
// routing table, it asks the kernel to make the exact decision the real
// socket would face. It returns the zero Addr when probing fails or when the
// answer is one of our own TUN addresses (which happens when the tunnel has
// captured the default route and would otherwise feed the probe back to us).
func probeSourceAddr(family int) netip.Addr {
	target := probeTargetIPv4
	network := "udp4"
	if family == FamilyIPv6 {
		target = probeTargetIPv6
		network = "udp6"
	}

	conn, err := net.DialUDP(network, nil, target)
	if err != nil {
		return netip.Addr{}
	}
	defer conn.Close()

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || local.IP == nil {
		return netip.Addr{}
	}
	parsed, ok := netip.AddrFromSlice(local.IP)
	if !ok {
		return netip.Addr{}
	}
	parsed = parsed.Unmap()
	if inTunPrefix(parsed) {
		return netip.Addr{}
	}
	return parsed
}

// choose resolves the best binding, preferring the interface whose address the
// kernel probe confirmed, then default-route ownership, then config preference.
// Interfaces without a usable source address are only used as a last resort,
// because binding them cannot produce a working socket anyway.
func choose(candidates []candidate, probe netip.Addr) (Binding, bool) {
	if binding, ok := chooseRanked(rank(candidates, probe), probe); ok {
		return binding, true
	}
	// 兜底：黑名单把所有候选都排除了（例如主机唯一的 uplink 恰好叫 wg0）。
	// 此时只放宽内置黑名单重新挑选，保证代理仍能出站而不是直接启动失败；
	// 用户显式配置的 ExcludeNames 仍然生效。
	return chooseRanked(rankIgnoringBlacklist(candidates), netip.Addr{})
}

func chooseRanked(ordered []candidate, probe netip.Addr) (Binding, bool) {
	for _, item := range ordered {
		if !item.usableSource {
			continue
		}
		reason := "default_route"
		switch {
		case probe.IsValid() && item.address == probe:
			reason = "kernel_route_probe"
		case !item.hasRoute:
			reason = "fallback_no_default_route"
		case item.virtual:
			reason = "default_route_virtual"
		}
		return Binding{
			InterfaceIndex: item.view.Index,
			InterfaceName:  item.view.Name,
			Address:        item.address,
			DefaultRoute:   item.hasRoute,
			Metric:         item.metric,
			Virtual:        item.virtual,
			Reason:         reason,
		}, true
	}
	return Binding{}, false
}

// rankIgnoringBlacklist ranks candidates with the built-in blacklist lifted
// while keeping user-configured exclusions in force.
func rankIgnoringBlacklist(candidates []candidate) []candidate {
	relaxed := make([]candidate, 0, len(candidates))
	for _, item := range candidates {
		if item.excluded && item.excludedBy == "builtin_blacklist" {
			item.excluded = false
		}
		relaxed = append(relaxed, item)
	}
	return rank(relaxed, netip.Addr{})
}

// systemViews snapshots the host's adapters.
func systemViews() []interfaceView {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	views := make([]interfaceView, 0, len(interfaces))
	for _, iface := range interfaces {
		view := interfaceView{
			Index:    iface.Index,
			Name:     iface.Name,
			Up:       iface.Flags&net.FlagUp != 0,
			Loopback: iface.Flags&net.FlagLoopback != 0,
		}
		if addrs, err := iface.Addrs(); err == nil {
			for _, addr := range addrs {
				switch typed := addr.(type) {
				case *net.IPNet:
					if parsed, ok := netip.AddrFromSlice(typed.IP); ok {
						view.Addresses = append(view.Addresses, parsed.Unmap())
					}
				case *net.IPAddr:
					if parsed, ok := netip.AddrFromSlice(typed.IP); ok {
						addr := parsed.Unmap()
						if typed.Zone != "" && addr.Is6() {
							addr = addr.WithZone(typed.Zone)
						}
						view.Addresses = append(view.Addresses, addr)
					}
				}
			}
		}
		views = append(views, view)
	}
	return views
}

// cacheTTL bounds how long a routing snapshot is reused. Adapter and route
// lookups cost syscalls per dial, but a TUN start/stop or a DHCP renewal must
// still be picked up quickly.
const cacheTTL = 10 * time.Second

type snapshot struct {
	views  []interfaceView
	routes map[int]map[int]uint32
	probes map[int]netip.Addr
}

var (
	cacheMu   sync.Mutex
	cache     *snapshot
	cacheTime time.Time
)

// InvalidateCache drops the cached routing snapshot. Call it after the TUN is
// started or stopped, since the adapter set changes at those points.
func InvalidateCache() {
	cacheMu.Lock()
	cache = nil
	cacheTime = time.Time{}
	cacheMu.Unlock()
}

// loadSnapshot returns a fresh-enough view of adapters, default routes and the
// kernel routing probe, refreshing at most once per cacheTTL.
func loadSnapshot() *snapshot {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cache != nil && time.Since(cacheTime) < cacheTTL {
		return cache
	}
	cache = &snapshot{
		views:  systemViews(),
		routes: map[int]map[int]uint32{},
		probes: map[int]netip.Addr{},
	}
	for _, family := range []int{FamilyIPv4, FamilyIPv6} {
		cache.routes[family] = defaultRouteMetrics(family)
		cache.probes[family] = probeSourceAddr(family)
	}
	cacheTime = time.Now()
	return cache
}

// Select returns the physical NIC that owns the default route for family,
// preferring an interface with a usable source address.
func Select(family int, cfg Config, logf func(string)) (Binding, error) {
	current := loadSnapshot()
	binding, ok := choose(collect(family, cfg, current.views, current.routes[family]), current.probes[family])
	if !ok {
		return Binding{}, fmt.Errorf("no usable outbound interface for family %d", family)
	}
	if logf != nil {
		logf("[netiface] selected " + binding.Describe())
	}
	return binding, nil
}

// SelectForTarget picks the family from addr and then selects a binding.
func SelectForTarget(addr string, cfg Config, logf func(string)) (Binding, error) {
	return Select(FamilyOf(addr), cfg, logf)
}

// Descriptor summarises one adapter for settings UIs.
type Descriptor struct {
	Name         string   `json:"name"`
	Index        int      `json:"index"`
	Up           bool     `json:"up"`
	Physical     bool     `json:"physical"`
	DefaultRoute bool     `json:"default_route"`
	IPv4         []string `json:"ipv4"`
	IPv6         []string `json:"ipv6"`
	// Usable reports whether the adapter can carry traffic at all (up, not
	// loopback, not our own tunnel).
	Usable bool `json:"usable"`
	// Selected reports whether the user pinned this adapter in config.
	Selected bool `json:"selected"`
	// CurrentOutbound reports that automatic selection would pick this adapter,
	// which is what the settings page labels as in use.
	CurrentOutbound bool `json:"current_outbound"`
	// Excluded reports whether the current config rules the adapter out, with
	// ExcludeReason naming the rule.
	Excluded      bool   `json:"excluded"`
	ExcludeReason string `json:"exclude_reason,omitempty"`
}

// DescribeAll returns every adapter using the default (unfiltered) config.
func DescribeAll() []Descriptor {
	return List(Config{})
}

// List returns every adapter with its addresses, physical/virtual verdict,
// default-route ownership, and whether the current config excludes it. The
// settings page renders this to offer a manual interface choice.
func List(cfg Config) []Descriptor {
	current := loadSnapshot()

	// An adapter owns a default route in either family.
	routeOwner := make(map[int]bool)
	for _, routes := range current.routes {
		for index := range routes {
			routeOwner[index] = true
		}
	}

	// The adapter automatic selection would actually use, so the settings page
	// can label it "in use" without repeating the ranking here.
	autoIndex := -1
	for _, family := range []int{FamilyIPv4, FamilyIPv6} {
		if binding, ok := choose(collect(family, cfg, current.views, current.routes[family]), current.probes[family]); ok {
			autoIndex = binding.InterfaceIndex
			break
		}
	}

	excluded := make(map[int]string)
	for _, family := range []int{FamilyIPv4, FamilyIPv6} {
		for _, item := range collect(family, cfg, current.views, current.routes[family]) {
			if item.excluded {
				if _, seen := excluded[item.view.Index]; !seen {
					excluded[item.view.Index] = item.excludedBy
				}
			}
		}
	}

	out := make([]Descriptor, 0, len(current.views))
	for _, view := range current.views {
		item := Descriptor{
			Name:         view.Name,
			Index:        view.Index,
			Up:           view.Up,
			Physical:     !IsVirtualAdapter(view.Name) && !IsOwnTunnel(view.Name) && !view.Loopback,
			DefaultRoute: routeOwner[view.Index],
			Usable:       view.Up && !view.Loopback && !IsOwnTunnel(view.Name),
			Selected:     isForced(cfg.ForceInterface, view.Name, view.Index),
		}
		item.CurrentOutbound = view.Index == autoIndex
		if reason, blocked := excluded[view.Index]; blocked {
			item.Excluded = true
			item.ExcludeReason = reason
		}
		for _, addr := range view.Addresses {
			if addr.Is4() {
				item.IPv4 = append(item.IPv4, addr.String())
			} else {
				item.IPv6 = append(item.IPv6, addr.String())
			}
		}
		out = append(out, item)
	}
	return out
}

// Explain returns a per-family ranking trace for diagnostics and bug reports.
func Explain(cfg Config) []string {
	current := loadSnapshot()
	out := make([]string, 0, len(current.views)*2)
	for _, family := range []int{FamilyIPv4, FamilyIPv6} {
		for _, item := range rank(collect(family, cfg, current.views, current.routes[family]), current.probes[family]) {
			addr := "none"
			if item.usableSource {
				addr = item.address.String()
			}
			out = append(out, fmt.Sprintf(
				"family=%d interface=%s index=%d address=%s default_route=%t metric=%d virtual=%t excluded=%t by=%s",
				family, item.view.Name, item.view.Index, addr, item.hasRoute, item.metric,
				item.virtual, item.excluded, item.excludedBy,
			))
		}
	}
	return out
}
