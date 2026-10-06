package singtun

import (
	"net"
	"net/netip"
	"sync"
)

const (
	// fake-ip 范围必须与 TUN 接口子网一致，确保 fake-ip 流量经过 TUN 被捕获。
	// TUN Inet4Address = 198.18.0.1/16, Inet6Address = fd65:198:18::1/64
	fakeIPv4Prefix = "198.18.0.0/16"
	fakeIPv6Prefix = "fd65:198:18::/64"
)

const (
	tunGateway4 = "198.18.0.1"
	tunDNS4     = "198.18.0.2"
	tunGateway6 = "fd65:198:18::1"
	tunDNS6     = "fd65:198:18::2"
)

var (
	reservedFakeIP4 = []netip.Addr{
		netip.MustParseAddr(tunGateway4),
		netip.MustParseAddr(tunDNS4),
	}
	reservedFakeIP6 = []netip.Addr{
		netip.MustParseAddr(tunGateway6),
		netip.MustParseAddr(tunDNS6),
	}
)

func isReservedFakeIP(addr netip.Addr, reserved []netip.Addr) bool {
	for _, r := range reserved {
		if addr == r {
			return true
		}
	}
	return false
}

// TUNGateway4 返回 TUN 的 IPv4 网关地址。它是 TUN 地址的唯一来源，
// 外部（如 core 的数据面自检）必须经此读取，不得再硬编码 "198.18.0.1"。
func TUNGateway4() string { return tunGateway4 }

func firstAllocatable(prefix netip.Prefix, reserved []netip.Addr) netip.Addr {
	addr := prefix.Addr().Next()
	for isReservedFakeIP(addr, reserved) {
		addr = addr.Next()
	}
	return addr
}

// FakeIPStore 管理 fake-ip ↔ 域名的双向映射
//
// 容量上限：maps 若不设上限，长时间运行会随访问过的域名数无限增长——
// addressCache 的键受 IP 范围限制会回绕覆盖，但 domainCache4/domainCache6
// 每个新域名都会新增一条永不删除的条目，是真实的内存泄漏。
// 超过上限时按 FIFO 淘汰最旧的域名（fake-ip 映射本就是短生命周期的访问缓存，
// 淘汰后该域名会重新分配新 IP，不影响已建立连接）。
const fakeIPStoreMaxEntries = 8192

type FakeIPStore struct {
	mu           sync.RWMutex
	addressCache map[netip.Addr]string // IP → 域名
	domainCache4 map[string]netip.Addr // 域名(IPv4) → IP
	domainCache6 map[string]netip.Addr // 域名(IPv6) → IP
	order4       []string              // domainCache4 的插入顺序，用于 FIFO 淘汰
	order6       []string              // domainCache6 的插入顺序，用于 FIFO 淘汰
	current4     netip.Addr            // IPv4 当前分配的 IP
	first4       netip.Addr            // IPv4 第一个可分配地址（跳过保留地址）
	last4        netip.Addr            // IPv4 范围最后一个 IP
	range4       netip.Prefix          // IPv4 范围
	current6     netip.Addr            // IPv6 当前分配的 IP
	first6       netip.Addr            // IPv6 第一个可分配地址（跳过保留地址）
	last6        netip.Addr            // IPv6 范围最后一个 IP
	range6       netip.Prefix          // IPv6 范围
}

// NewFakeIPStore 创建新的 fake-ip 存储
func NewFakeIPStore() *FakeIPStore {
	range4 := netip.MustParsePrefix(fakeIPv4Prefix)
	lastAddr4 := broadcastAddr(range4)

	range6 := netip.MustParsePrefix(fakeIPv6Prefix)
	lastAddr6 := broadcastAddr(range6)

	return &FakeIPStore{
		addressCache: make(map[netip.Addr]string),
		domainCache4: make(map[string]netip.Addr),
		domainCache6: make(map[string]netip.Addr),
		order4:       make([]string, 0, fakeIPStoreMaxEntries/8),
		order6:       make([]string, 0, fakeIPStoreMaxEntries/8),
		current4:     firstAllocatable(range4, reservedFakeIP4),
		first4:       firstAllocatable(range4, reservedFakeIP4),
		last4:        lastAddr4,
		range4:       range4,
		current6:     firstAllocatable(range6, reservedFakeIP6),
		first6:       firstAllocatable(range6, reservedFakeIP6),
		last6:        lastAddr6,
		range6:       range6,
	}
}

// Create 为域名分配一个假 IPv4 地址（去重）
// 返回 (ip, isNew)，isNew=true 表示本次新建，false 表示命中缓存
func (s *FakeIPStore) Create(domain string) (netip.Addr, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有映射（去重）
	if ip, ok := s.domainCache4[domain]; ok {
		return ip, false
	}

	// 分配下一个 IP
	ip := s.current4
	s.current4 = s.current4.Next()

	// 环形回绕
	if !s.range4.Contains(s.current4) || s.current4 == s.last4 {
		s.current4 = s.first4
	}

	// IP 被重新分配：旧域名的正向映射必须一并失效，
	// 否则该域名的 Create 会命中 domainCache4 返回过期 IP。
	if previous, ok := s.addressCache[ip]; ok && previous != domain {
		delete(s.domainCache4, previous)
	}

	// 存储双向映射
	s.addressCache[ip] = domain
	s.domainCache4[domain] = ip
	s.order4 = append(s.order4, domain)
	s.evict4Locked()

	return ip, true
}

// CreateIPv6 为域名分配一个假 IPv6 地址（去重）
// 返回 (ip, isNew)，isNew=true 表示本次新建，false 表示命中缓存
func (s *FakeIPStore) CreateIPv6(domain string) (netip.Addr, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已有映射（去重）
	if ip, ok := s.domainCache6[domain]; ok {
		return ip, false
	}

	// 分配下一个 IP
	ip := s.current6
	s.current6 = s.current6.Next()

	// 环形回绕
	if !s.range6.Contains(s.current6) || s.current6 == s.last6 {
		s.current6 = s.first6
	}

	if previous, ok := s.addressCache[ip]; ok && previous != domain {
		delete(s.domainCache6, previous)
	}

	// 存储双向映射
	s.addressCache[ip] = domain
	s.domainCache6[domain] = ip
	s.order6 = append(s.order6, domain)
	s.evict6Locked()

	return ip, true
}

// evict4Locked 超出上限时按 FIFO 淘汰最旧的 IPv4 域名映射，调用方须持有写锁。
func (s *FakeIPStore) evict4Locked() {
	for len(s.order4) > fakeIPStoreMaxEntries {
		oldest := s.order4[0]
		s.order4 = s.order4[1:]
		ip, ok := s.domainCache4[oldest]
		if !ok {
			continue
		}
		delete(s.domainCache4, oldest)
		// 仅当该 IP 仍指向被淘汰的域名时才删（可能已被回绕覆盖给新域名）
		if current, ok := s.addressCache[ip]; ok && current == oldest {
			delete(s.addressCache, ip)
		}
	}
}

// evict6Locked 超出上限时按 FIFO 淘汰最旧的 IPv6 域名映射，调用方须持有写锁。
func (s *FakeIPStore) evict6Locked() {
	for len(s.order6) > fakeIPStoreMaxEntries {
		oldest := s.order6[0]
		s.order6 = s.order6[1:]
		ip, ok := s.domainCache6[oldest]
		if !ok {
			continue
		}
		delete(s.domainCache6, oldest)
		if current, ok := s.addressCache[ip]; ok && current == oldest {
			delete(s.addressCache, ip)
		}
	}
}

// Lookup 通过假 IP 反查域名
func (s *FakeIPStore) Lookup(ip netip.Addr) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	domain, ok := s.addressCache[ip]
	return domain, ok
}

// Contains 检查 IP 是否在 fake-ip 范围内
func (s *FakeIPStore) Contains(ip netip.Addr) bool {
	if ip.Is4() {
		if isReservedFakeIP(ip, reservedFakeIP4) {
			return false
		}
		return s.range4.Contains(ip)
	}
	if isReservedFakeIP(ip, reservedFakeIP6) {
		return false
	}
	return s.range6.Contains(ip)
}

// AsNetIP 将 netip.Addr 转换为 net.IP
func AsNetIP(addr netip.Addr) net.IP {
	if addr.Is4() {
		b := addr.As4()
		return net.IP(b[:])
	}
	b := addr.As16()
	return net.IP(b[:])
}

// broadcastAddr 计算范围的广播地址（最后一个可用地址）
func broadcastAddr(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()
	if addr.Is4() {
		b := addr.As4()
		mask := net.CIDRMask(prefix.Bits(), 32)
		for i := 0; i < 4; i++ {
			b[i] = b[i] | ^mask[i]
		}
		return netip.AddrFrom4(b)
	}
	b := addr.As16()
	mask := net.CIDRMask(prefix.Bits(), 128)
	for i := 0; i < 16; i++ {
		b[i] = b[i] | ^mask[i]
	}
	return netip.AddrFrom16(b)
}
