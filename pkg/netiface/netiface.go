package netiface

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	FamilyIPv4 = 4
	FamilyIPv6 = 6

	selectionCacheTTL = 10 * time.Second
)

var ErrNoUsableInterface = errors.New("netiface: no usable outbound interface")

var tunAddressPrefixes = []netip.Prefix{
	netip.MustParsePrefix("198.18.0.0/16"),
	netip.MustParsePrefix("fd65:198:18::/64"),
}

var ownTunnelMarkers = []string{
	"snishaper",
	"sing-box",
	"singbox",
	"singtun",
	"wintun",
}

// virtualExactNames 是完全匹配即判定的虚拟网卡名，用于 lo / br0 / meta 这类
// 过短的名字：改用子串匹配会误伤物理网卡（Local Area Connection 含 "lo"）。
var virtualExactNames = map[string]struct{}{
	"lo":           {},
	"lo0":          {},
	"br0":          {},
	"bond0":        {},
	"dummy0":       {},
	"vnet0":        {},
	"vmnet1":       {},
	"vmnet8":       {},
	"vboxnet0":     {},
	"virbr0":       {},
	"virbr0-nic":   {},
	"docker0":      {},
	"cni0":         {},
	"flannel.1":    {},
	"kube-ipvs0":   {},
	"tunl0":        {},
	"vxlan.calico": {},
	"gif0":         {},
	"stf0":         {},
	"bridge0":      {},
	"awdl0":        {},
	"llw0":         {},
	"p2p0":         {},
	"vmenet0":      {},
	"meta":         {},
	"mihomo":       {},
	"clash":        {},
	"sing-box":     {},
	"v2rayn":       {},
	"nekoray":      {},
	"wireguard":    {},
	"tailscale":    {},
	"zerotier":     {},
}

// virtualNamePrefixes 命中即判定的前缀族（序号/随机后缀不限）。
var virtualNamePrefixes = []string{
	"utun",
	"veth",
	"br-",
	"cali",
	"cilium_",
	"vxlan",
	"flannel",
	"singbox",
	"singtun",
}

// virtualNumericPrefixes 仅在「前缀 + 纯数字后缀」时判定，避免 ppp/br/cni 这类
// 短前缀误伤物理网卡。
var virtualNumericPrefixes = []string{
	"lo",
	"br",
	"bond",
	"dummy",
	"vnet",
	"vmnet",
	"vmenet",
	"vboxnet",
	"virbr",
	"cni",
	"zt",
	"wg",
	"ppp",
	"tun",
	"tap",
	"warp",
	"gif",
	"stf",
	"bridge",
	"awdl",
	"llw",
	"p2p",
	"iptun",
}

var virtualNameMarkers = []string{
	"vethernet",
	"hyper-v",
	"hyperv",
	"default switch",
	"wsl",
	"vmware",
	"virtualbox",
	"vbox",
	"docker",
	"podman",
	"virbr",
	"wireguard",
	"tailscale",
	"zerotier",
	"openvpn",
	"nordvpn",
	"proton",
	"surfshark",
	"expressvpn",
	"windscribe",
	"mullvad",
	"cloudflare warp",
	"cloudflarewarp",
	"warp-cli",
	"npcap",
	"km-test",
	"loopback",
	"anyconnect",
	"virtual miniport",
	"fortinet",
	"forticlient",
	"ssl vpn",
	"sangfor",
	"easyconnect",
	"softether",
	"vpn client adapter",
	"vpn adapter",
	"globalprotect",
	"clash",
	"mihomo",
	"shadowsocks",
	"v2ray",
	"xray",
	"hamachi",
	"radmin",
	"bluetooth",
	"teredo",
	"isatap",
	"6to4",
	"virtual",
	"vpn",
	"host-only",
	"环回适配器",
	"蓝牙",
}

type Config struct {
	ExcludeInterfaces []string
	IncludeInterfaces []string
	ExcludeAddresses  []string
	ForceInterface    string
}

type normalizedConfig struct {
	excludeInterfaces []string
	includeInterfaces []string
	excludeAddresses  []netip.Prefix
	forceInterface    string
	key               string
}

type defaultRoute struct {
	interfaceIndex int
	gateway        netip.Addr
	metric         uint32
	hasMetric      bool
	disableDefault bool
}

type InterfaceView struct {
	Name      string
	Index     int
	Up        bool
	Loopback  bool
	Addresses []netip.Addr
}

type Candidate struct {
	InterfaceName       string
	InterfaceIndex      int
	Address             netip.Addr
	Gateway             netip.Addr
	Metric              uint32
	HasDefaultRoute     bool
	DisableDefaultRoute bool
	Untrusted           bool
	Excluded            bool
	ExcludedBy          string
	ForcedInclude       bool
}

type Binding struct {
	Address        netip.Addr
	InterfaceName  string
	InterfaceIndex int
	Gateway        netip.Addr
	Metric         uint32
	Trusted        bool
	DefaultRoute   bool
	Mode           string
}

func (b Binding) LocalTCPAddr() *net.TCPAddr {
	addr := &net.TCPAddr{IP: net.IP(b.Address.AsSlice())}
	if b.Address.Is6() && b.Address.IsLinkLocalUnicast() {
		addr.Zone = b.Address.Zone()
	}
	return addr
}

func (b Binding) LocalUDPAddr() *net.UDPAddr {
	addr := &net.UDPAddr{IP: net.IP(b.Address.AsSlice())}
	if b.Address.Is6() && b.Address.IsLinkLocalUnicast() {
		addr.Zone = b.Address.Zone()
	}
	return addr
}

func (b Binding) Describe() string {
	gateway := "none"
	if b.Gateway.IsValid() {
		gateway = b.Gateway.String()
	}
	return fmt.Sprintf(
		"interface=%s index=%d address=%s gateway=%s metric=%d physical=%t default_route=%t source=%s",
		b.InterfaceName, b.InterfaceIndex, b.Address, gateway, b.Metric, b.Trusted, b.DefaultRoute, b.Mode,
	)
}

func (c Candidate) describe() string {
	gateway := "none"
	if c.Gateway.IsValid() {
		gateway = c.Gateway.String()
	}
	physical := "yes"
	if c.Untrusted {
		physical = "no"
	}
	return fmt.Sprintf("%s(idx=%d,addr=%s,gw=%s,metric=%d,route=%t,physical=%s)",
		c.InterfaceName, c.InterfaceIndex, c.Address, gateway, c.Metric, c.HasDefaultRoute, physical)
}

func (c Config) normalize() normalizedConfig {
	out := normalizedConfig{
		excludeInterfaces: normalizePatterns(c.ExcludeInterfaces),
		includeInterfaces: normalizePatterns(c.IncludeInterfaces),
		forceInterface:    strings.ToLower(strings.TrimSpace(c.ForceInterface)),
	}
	for _, raw := range c.ExcludeAddresses {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if !strings.Contains(value, "/") {
			if addr, err := netip.ParseAddr(value); err == nil {
				bits := 32
				if addr.Is6() {
					bits = 128
				}
				out.excludeAddresses = append(out.excludeAddresses, netip.PrefixFrom(addr.Unmap(), bits))
				continue
			}
		}
		if prefix, err := netip.ParsePrefix(value); err == nil {
			out.excludeAddresses = append(out.excludeAddresses, prefix.Masked())
		}
	}
	out.key = fmt.Sprintf("%v|%v|%v|%s", out.excludeInterfaces, out.includeInterfaces, out.excludeAddresses, out.forceInterface)
	return out
}

func normalizePatterns(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func matchesPattern(patterns []string, name string, index int) (string, bool) {
	lowered := strings.ToLower(name)
	indexText := strconv.Itoa(index)
	for _, pattern := range patterns {
		if pattern == lowered || pattern == indexText {
			return pattern, true
		}
		if ok, err := path.Match(pattern, lowered); err == nil && ok {
			return pattern, true
		}
		if strings.Contains(lowered, pattern) {
			return pattern, true
		}
	}
	return "", false
}

func matchesExactPattern(name string, index int, pattern string) bool {
	lowered := strings.ToLower(name)
	if pattern == "" {
		return false
	}
	if pattern == lowered || pattern == strconv.Itoa(index) {
		return true
	}
	ok, err := path.Match(pattern, lowered)
	return err == nil && ok
}

func containsMarker(markers []string, name string) bool {
	lowered := strings.ToLower(name)
	for _, marker := range markers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

func IsOwnTunnel(name string) bool {
	return containsMarker(ownTunnelMarkers, name)
}

func isNumericSuffix(rest string) bool {
	for _, char := range rest {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func IsVirtualAdapter(name string) bool {
	if IsOwnTunnel(name) {
		return true
	}
	lowered := strings.ToLower(strings.TrimSpace(name))
	if lowered == "" {
		return false
	}
	if _, ok := virtualExactNames[lowered]; ok {
		return true
	}
	for _, prefix := range virtualNamePrefixes {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	for _, prefix := range virtualNumericPrefixes {
		if rest, ok := strings.CutPrefix(lowered, prefix); ok && isNumericSuffix(rest) {
			return true
		}
	}
	return containsMarker(virtualNameMarkers, lowered)
}

func inTunPrefix(addr netip.Addr) bool {
	for _, prefix := range tunAddressPrefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func usableAddress(addr netip.Addr, family int) bool {
	if family == FamilyIPv4 && !addr.Is4() {
		return false
	}
	if family == FamilyIPv6 && !addr.Is6() {
		return false
	}
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	return !inTunPrefix(addr)
}

func selectAddress(view InterfaceView, family int) (netip.Addr, bool) {
	for _, addr := range view.Addresses {
		if usableAddress(addr, family) {
			return addr, true
		}
	}
	return netip.Addr{}, false
}

func collect(family int, cfg normalizedConfig, views []InterfaceView, routes []defaultRoute) []Candidate {
	bestRoute := make(map[int]defaultRoute, len(routes))
	for _, route := range routes {
		if route.gateway.IsValid() && FamilyOf(route.gateway) != family {
			continue
		}
		previous, ok := bestRoute[route.interfaceIndex]
		if !ok || (route.hasMetric && route.metric < previous.metric) {
			bestRoute[route.interfaceIndex] = route
		}
	}

	candidates := make([]Candidate, 0, len(views))
	for _, view := range views {
		if !view.Up || view.Loopback || IsOwnTunnel(view.Name) {
			continue
		}
		addr, ok := selectAddress(view, family)
		if !ok {
			continue
		}

		candidate := Candidate{
			InterfaceName:  view.Name,
			InterfaceIndex: view.Index,
			Address:        addr,
			Untrusted:      IsVirtualAdapter(view.Name),
		}

		if route, ok := bestRoute[view.Index]; ok {
			candidate.HasDefaultRoute = true
			candidate.Gateway = route.gateway
			candidate.DisableDefaultRoute = route.disableDefault
			if route.hasMetric {
				candidate.Metric = route.metric
			}
			if route.disableDefault {
				candidate.Untrusted = true
			}
		}

		if cfg.forceInterface != "" {
			if !matchesExactPattern(view.Name, view.Index, cfg.forceInterface) {
				candidate.Excluded = true
				candidate.ExcludedBy = "interface_name=" + cfg.forceInterface
				candidates = append(candidates, candidate)
				continue
			}
			candidate.Untrusted = false
			candidate.ForcedInclude = true
			candidates = append(candidates, candidate)
			continue
		}

		if _, ok := matchesPattern(cfg.includeInterfaces, view.Name, view.Index); ok {
			candidate.Untrusted = false
			candidate.ForcedInclude = true
			candidates = append(candidates, candidate)
			continue
		}

		if pattern, ok := matchesPattern(cfg.excludeInterfaces, view.Name, view.Index); ok {
			candidate.Excluded = true
			candidate.ExcludedBy = "exclude_interface=" + pattern
			candidates = append(candidates, candidate)
			continue
		}

		for _, prefix := range cfg.excludeAddresses {
			if prefix.Contains(addr) {
				candidate.Excluded = true
				candidate.ExcludedBy = "route_exclude_address=" + prefix.String()
				break
			}
		}

		candidates = append(candidates, candidate)
	}
	return candidates
}

func rankCandidates(candidates []Candidate) []Candidate {
	ranked := make([]Candidate, len(candidates))
	copy(ranked, candidates)
	sort.SliceStable(ranked, func(i, j int) bool {
		left, right := ranked[i], ranked[j]
		if left.HasDefaultRoute != right.HasDefaultRoute {
			return left.HasDefaultRoute
		}
		if left.Metric != right.Metric {
			return left.Metric < right.Metric
		}
		if left.InterfaceIndex != right.InterfaceIndex {
			return left.InterfaceIndex < right.InterfaceIndex
		}
		return left.Address.Less(right.Address)
	})
	return ranked
}

var probeCandidateRoute = defaultProbe

func defaultProbe(candidate Candidate) error {
	if !candidate.Gateway.IsValid() {
		return nil
	}
	network := "udp4"
	if candidate.Address.Is6() {
		network = "udp6"
	}
	local := &net.UDPAddr{IP: net.IP(candidate.Address.AsSlice())}
	remote := &net.UDPAddr{IP: net.IP(candidate.Gateway.AsSlice())}
	if candidate.Gateway.Is6() && candidate.Gateway.IsLinkLocalUnicast() {
		zone := candidate.Gateway.Zone()
		if zone == "" {
			zone = strconv.Itoa(candidate.InterfaceIndex)
		}
		remote.Zone = zone
	}
	conn, err := net.DialUDP(network, local, remote)
	if err != nil {
		return err
	}
	return conn.Close()
}

func bindingFrom(candidate Candidate, source string) *Binding {
	return &Binding{
		Address:        candidate.Address,
		InterfaceName:  candidate.InterfaceName,
		InterfaceIndex: candidate.InterfaceIndex,
		Gateway:        candidate.Gateway,
		Metric:         candidate.Metric,
		Trusted:        !candidate.Untrusted,
		DefaultRoute:   candidate.HasDefaultRoute,
		Mode:           source,
	}
}

func attempt(group []Candidate, source string, strict bool, logf func(string)) *Binding {
	if len(group) == 0 {
		return nil
	}
	ranked := rankCandidates(group)
	logf("[netiface] candidates ranked: " + joinCandidates(ranked, 3))
	for i := range ranked {
		candidate := ranked[i]
		if err := probeCandidateRoute(candidate); err != nil {
			logf(fmt.Sprintf("[netiface] candidate %s rejected by probe: %v", candidate.InterfaceName, err))
			continue
		}
		return bindingFrom(candidate, source)
	}
	if strict {
		return nil
	}
	top := ranked[0]
	logf("[netiface] probe rejected every candidate, using top-ranked " + top.InterfaceName + " as last resort")
	return bindingFrom(top, source+"-probe-failed")
}

func joinCandidates(candidates []Candidate, limit int) string {
	if len(candidates) < limit {
		limit = len(candidates)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, candidates[i].describe())
	}
	if len(candidates) > limit {
		parts = append(parts, fmt.Sprintf("(+%d more)", len(candidates)-limit))
	}
	return strings.Join(parts, " ")
}

var ErrForcedInterfaceUnusable = errors.New("netiface: selected interface is not usable")

func pickSmart(candidates []Candidate, family int, logf func(string)) (*Binding, error) {
	preferred := make([]Candidate, 0, len(candidates))
	rejected := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Excluded {
			rejected = append(rejected, candidate)
			continue
		}
		preferred = append(preferred, candidate)
	}
	if len(preferred) == 0 {
		reason := "config"
		if len(rejected) > 0 {
			reason = rejected[0].ExcludedBy
		}
		logf(fmt.Sprintf("[netiface] every candidate was excluded (%s), ignoring exclusions for ipv%d", reason, family))
		preferred = rejected
	}

	trusted := make([]Candidate, 0, len(preferred))
	untrusted := make([]Candidate, 0, len(preferred))
	for _, candidate := range preferred {
		if candidate.Untrusted {
			untrusted = append(untrusted, candidate)
			continue
		}
		trusted = append(trusted, candidate)
	}

	if binding := attempt(trusted, "route-table", false, logf); binding != nil {
		logf("[netiface] outbound binding selected: " + binding.Describe())
		return binding, nil
	}
	if len(untrusted) > 0 {
		logf("[netiface] no physical interface available for ipv" + strconv.Itoa(family) + ", falling back to virtual adapter")
		if binding := attempt(untrusted, "route-table-virtual-fallback", false, logf); binding != nil {
			logf("[netiface] outbound binding selected: " + binding.Describe())
			return binding, nil
		}
	}
	return nil, ErrNoUsableInterface
}

func pickForced(candidates []Candidate, forced string, logf func(string)) (*Binding, error) {
	preferred := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Excluded {
			preferred = append(preferred, candidate)
		}
	}
	if len(preferred) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrForcedInterfaceUnusable, forced)
	}
	binding := attempt(preferred, "user-selected", true, logf)
	if binding == nil {
		return nil, fmt.Errorf("%w: %s", ErrForcedInterfaceUnusable, forced)
	}
	logf("[netiface] user-selected outbound binding: " + binding.Describe())
	return binding, nil
}

var lookupRoutes = lookupDefaultRoutes

func chooseFrom(views []InterfaceView, family int, cfg normalizedConfig, logf func(string)) (*Binding, error) {
	routes := lookupRoutes(family)

	if cfg.forceInterface != "" {
		candidates := collect(family, cfg, views, routes)
		binding, err := pickForced(candidates, cfg.forceInterface, logf)
		if err == nil {
			return binding, nil
		}
		logf(fmt.Sprintf("[netiface] %v, falling back to smart selection", err))
		cfg.forceInterface = ""
	}

	candidates := collect(family, cfg, views, routes)
	if len(candidates) == 0 {
		return nil, ErrNoUsableInterface
	}
	return pickSmart(candidates, family, logf)
}

func systemInterfaces() []InterfaceView {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	views := make([]InterfaceView, 0, len(interfaces))
	for _, iface := range interfaces {
		view := InterfaceView{
			Name:     iface.Name,
			Index:    iface.Index,
			Up:       iface.Flags&net.FlagUp != 0,
			Loopback: iface.Flags&net.FlagLoopback != 0,
		}
		if addrs, err := iface.Addrs(); err == nil {
			for _, raw := range addrs {
				ipNet, ok := raw.(*net.IPNet)
				if !ok || ipNet.IP == nil {
					continue
				}
				if addr, ok := netip.AddrFromSlice(ipNet.IP); ok {
					view.Addresses = append(view.Addresses, addr.Unmap())
				}
			}
		}
		views = append(views, view)
	}
	return views
}

type cacheEntry struct {
	binding *Binding
	err     error
	at      time.Time
}

var (
	cacheMu sync.Mutex
	cache   = make(map[string]cacheEntry)
)

func cacheKey(family int, cfg normalizedConfig) string {
	return strconv.Itoa(family) + "|" + cfg.key
}

func Select(family int, cfg Config, logf func(string)) (*Binding, error) {
	normalized := cfg.normalize()
	key := cacheKey(family, normalized)

	cacheMu.Lock()
	entry, ok := cache[key]
	if ok && time.Since(entry.at) < selectionCacheTTL {
		cacheMu.Unlock()
		if entry.err != nil {
			return nil, entry.err
		}
		return entry.binding, nil
	}
	cacheMu.Unlock()

	if logf == nil {
		logf = func(string) {}
	}
	binding, err := chooseFrom(systemInterfaces(), family, normalized, logf)

	cacheMu.Lock()
	for k, v := range cache {
		if time.Since(v.at) >= selectionCacheTTL {
			delete(cache, k)
		}
	}
	cache[key] = cacheEntry{binding: binding, err: err, at: time.Now()}
	cacheMu.Unlock()

	return binding, err
}

func InvalidateCache() {
	cacheMu.Lock()
	cache = make(map[string]cacheEntry)
	cacheMu.Unlock()
}

func FamilyOf(addr netip.Addr) int {
	if addr.Is6() {
		return FamilyIPv6
	}
	return FamilyIPv4
}

func SelectForAddr(addr netip.Addr, cfg Config, logf func(string)) (*Binding, error) {
	return Select(FamilyOf(addr), cfg, logf)
}

func SelectForTarget(target string, cfg Config, logf func(string)) (*Binding, error) {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		host = target
	}
	parsed, err := netip.ParseAddr(host)
	if err != nil {
		return Select(FamilyIPv4, cfg, logf)
	}
	return Select(FamilyOf(parsed), cfg, logf)
}

type Descriptor struct {
	Name          string   `json:"name"`
	Index         int      `json:"index"`
	Addresses     []string `json:"addresses"`
	Up            bool     `json:"up"`
	Virtual       bool     `json:"virtual"`
	OwnTunnel     bool     `json:"own_tunnel"`
	DefaultRoute  bool     `json:"default_route"`
	Gateway       string   `json:"gateway,omitempty"`
	Metric        uint32   `json:"metric"`
	SmartSelected bool     `json:"smart_selected"`
}

func List(cfg Config) []Descriptor {
	normalized := cfg.normalize()
	normalized.forceInterface = ""
	views := systemInterfaces()

	// 每个地址族只查一次路由表：lookupRoutes 在 Windows 上要读整张
	// IP 转发表并对每个接口调 GetIpInterfaceEntry，重复调用会让
	// GetNetworkInterfaces 明显变慢。
	routesByFamily := make(map[int][]defaultRoute, 2)
	for _, family := range []int{FamilyIPv4, FamilyIPv6} {
		routesByFamily[family] = lookupRoutes(family)
	}

	routesByIndex := make(map[int]defaultRoute)
	for _, family := range []int{FamilyIPv4, FamilyIPv6} {
		for _, route := range routesByFamily[family] {
			previous, ok := routesByIndex[route.interfaceIndex]
			if !ok || (route.hasMetric && route.metric < previous.metric) {
				routesByIndex[route.interfaceIndex] = route
			}
		}
	}

	smart := make(map[int]bool, 2)
	for _, family := range []int{FamilyIPv4, FamilyIPv6} {
		binding, err := pickSmart(collect(family, normalized, views, routesByFamily[family]), family, func(string) {})
		if err == nil && binding != nil {
			smart[binding.InterfaceIndex] = true
		}
	}

	out := make([]Descriptor, 0, len(views))
	for _, view := range views {
		if view.Loopback {
			continue
		}
		descriptor := Descriptor{
			Name:          view.Name,
			Index:         view.Index,
			Up:            view.Up,
			Virtual:       IsVirtualAdapter(view.Name),
			OwnTunnel:     IsOwnTunnel(view.Name),
			SmartSelected: smart[view.Index],
		}
		for _, addr := range view.Addresses {
			if addr.IsLinkLocalUnicast() || addr.IsLoopback() {
				continue
			}
			descriptor.Addresses = append(descriptor.Addresses, addr.String())
		}
		if route, ok := routesByIndex[view.Index]; ok {
			descriptor.DefaultRoute = true
			descriptor.Metric = route.metric
			if route.gateway.IsValid() {
				descriptor.Gateway = route.gateway.String()
			}
		}
		out = append(out, descriptor)
	}

	sort.SliceStable(out, func(i, j int) bool {
		left, right := out[i], out[j]
		if left.Virtual != right.Virtual {
			return !left.Virtual
		}
		if left.DefaultRoute != right.DefaultRoute {
			return left.DefaultRoute
		}
		if left.Metric != right.Metric {
			return left.Metric < right.Metric
		}
		return left.Index < right.Index
	})
	return out
}
