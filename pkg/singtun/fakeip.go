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

// FakeIPStore 管理 fake-ip ↔ 域名的双向映射
type FakeIPStore struct {
	mu           sync.RWMutex
	addressCache map[netip.Addr]string // IP → 域名
	domainCache4 map[string]netip.Addr // 域名(IPv4) → IP
	domainCache6 map[string]netip.Addr // 域名(IPv6) → IP
	current4     netip.Addr            // IPv4 当前分配的 IP
	last4        netip.Addr            // IPv4 范围最后一个 IP
	range4       netip.Prefix          // IPv4 范围
	current6     netip.Addr            // IPv6 当前分配的 IP
	last6        netip.Addr            // IPv6 范围最后一个 IP
	range6       netip.Prefix          // IPv6 范围
}

// NewFakeIPStore 创建新的 fake-ip 存储
func NewFakeIPStore() *FakeIPStore {
	// IPv4 范围
	range4 := netip.MustParsePrefix(fakeIPv4Prefix)
	startAddr4 := range4.Addr().Next().Next()
	lastAddr4 := broadcastAddr(range4)

	// IPv6 范围
	range6 := netip.MustParsePrefix(fakeIPv6Prefix)
	startAddr6 := range6.Addr().Next().Next()
	lastAddr6 := broadcastAddr(range6)

	return &FakeIPStore{
		addressCache: make(map[netip.Addr]string),
		domainCache4: make(map[string]netip.Addr),
		domainCache6: make(map[string]netip.Addr),
		current4:     startAddr4,
		last4:        lastAddr4,
		range4:       range4,
		current6:     startAddr6,
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
		s.current4 = s.range4.Addr().Next().Next()
	}

	// 存储双向映射
	s.addressCache[ip] = domain
	s.domainCache4[domain] = ip

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
		s.current6 = s.range6.Addr().Next().Next()
	}

	// 存储双向映射
	s.addressCache[ip] = domain
	s.domainCache6[domain] = ip

	return ip, true
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
		return s.range4.Contains(ip)
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
	return addr // IPv6 暂不处理
}
