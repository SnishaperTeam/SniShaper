package singtun

import (
	"fmt"
	"net/netip"
	"testing"
)

func TestFakeIPStoreDeduplicatesDomain(t *testing.T) {
	store := NewFakeIPStore()

	first, isNew := store.Create("example.com.")
	if !isNew {
		t.Fatal("first Create should report isNew=true")
	}
	second, isNew := store.Create("example.com.")
	if isNew {
		t.Fatal("second Create for same domain should report isNew=false")
	}
	if first != second {
		t.Fatalf("same domain must map to same fake-ip: %s vs %s", first, second)
	}
	if domain, ok := store.Lookup(first); !ok || domain != "example.com." {
		t.Fatalf("Lookup(%s) = %q, %v; want example.com., true", first, domain, ok)
	}
}

func TestFakeIPStoreLookupRoundTripIPv6(t *testing.T) {
	store := NewFakeIPStore()

	ip, isNew := store.CreateIPv6("v6.example.com.")
	if !isNew {
		t.Fatal("first CreateIPv6 should report isNew=true")
	}
	if !ip.Is6() {
		t.Fatalf("CreateIPv6 returned non-IPv6 address %s", ip)
	}
	domain, ok := store.Lookup(ip)
	if !ok || domain != "v6.example.com." {
		t.Fatalf("Lookup(%s) = %q, %v; want v6.example.com., true", ip, domain, ok)
	}
}

func TestFakeIPStoreContainsRespectsFamily(t *testing.T) {
	store := NewFakeIPStore()

	v4, _ := store.Create("a.example.com.")
	v6, _ := store.CreateIPv6("a.example.com.")

	if !store.Contains(v4) {
		t.Fatalf("Contains(%s) = false; want true", v4)
	}
	if !store.Contains(v6) {
		t.Fatalf("Contains(%s) = false; want true", v6)
	}
}

// TUN 网关与 DNS 地址绝不能进入 fake-ip 池，否则分配到 198.18.0.2 的域名
// 会与 DNS 劫持入口冲突，产生 "fake-ip has no domain mapping" 刷屏。
func TestFakeIPStoreSkipsReservedAddresses(t *testing.T) {
	store := NewFakeIPStore()

	gateway4 := netip.MustParseAddr("198.18.0.1")
	dns4 := netip.MustParseAddr("198.18.0.2")
	gateway6 := netip.MustParseAddr("fd65:198:18::1")
	dns6 := netip.MustParseAddr("fd65:198:18::2")

	first, _ := store.Create("first.example.com.")
	if first == gateway4 || first == dns4 {
		t.Fatalf("first allocation collided with reserved TUN address: %s", first)
	}

	sixth, _ := store.CreateIPv6("first-v6.example.com.")
	if sixth == gateway6 || sixth == dns6 {
		t.Fatalf("first IPv6 allocation collided with reserved TUN address: %s", sixth)
	}

	for _, reserved := range []netip.Addr{gateway4, dns4, gateway6, dns6} {
		if store.Contains(reserved) {
			t.Fatalf("reserved TUN address %s must not be treated as fake-ip", reserved)
		}
		if _, ok := store.Lookup(reserved); ok {
			t.Fatalf("reserved TUN address %s must not resolve to a domain", reserved)
		}
	}
}

// domainCache4/domainCache6 必须受上限约束，否则长时间运行会无限增长。
func TestFakeIPStoreEvictsBeyondCapacity(t *testing.T) {
	store := NewFakeIPStore()

	total := fakeIPStoreMaxEntries + 512
	for i := 0; i < total; i++ {
		store.Create(fmt.Sprintf("domain-%d.example.com.", i))
	}

	store.mu.RLock()
	count4 := len(store.domainCache4)
	order4 := len(store.order4)
	store.mu.RUnlock()

	if count4 > fakeIPStoreMaxEntries {
		t.Fatalf("domainCache4 grew to %d entries, limit is %d", count4, fakeIPStoreMaxEntries)
	}
	if order4 > fakeIPStoreMaxEntries {
		t.Fatalf("order4 grew to %d entries, limit is %d", order4, fakeIPStoreMaxEntries)
	}
}

func TestFakeIPStoreEvictsIPv6BeyondCapacity(t *testing.T) {
	store := NewFakeIPStore()

	total := fakeIPStoreMaxEntries + 256
	for i := 0; i < total; i++ {
		store.CreateIPv6(fmt.Sprintf("v6-%d.example.com.", i))
	}

	store.mu.RLock()
	count6 := len(store.domainCache6)
	order6 := len(store.order6)
	store.mu.RUnlock()

	if count6 > fakeIPStoreMaxEntries {
		t.Fatalf("domainCache6 grew to %d entries, limit is %d", count6, fakeIPStoreMaxEntries)
	}
	if order6 > fakeIPStoreMaxEntries {
		t.Fatalf("order6 grew to %d entries, limit is %d", order6, fakeIPStoreMaxEntries)
	}
}

// 淘汰必须只清理最旧条目，最近写入的映射仍可命中。
func TestFakeIPStoreKeepsRecentEntriesAfterEviction(t *testing.T) {
	store := NewFakeIPStore()

	for i := 0; i < fakeIPStoreMaxEntries+64; i++ {
		store.Create(fmt.Sprintf("old-%d.example.com.", i))
	}
	recent, _ := store.Create("recent.example.com.")

	domain, ok := store.Lookup(recent)
	if !ok || domain != "recent.example.com." {
		t.Fatalf("recent domain lost after eviction: Lookup(%s) = %q, %v", recent, domain, ok)
	}
}

// addressCache 反向映射不得残留指向已被淘汰域名的条目。
func TestFakeIPStoreEvictionRemovesStaleReverseMapping(t *testing.T) {
	store := NewFakeIPStore()

	first, _ := store.Create("victim.example.com.")
	for i := 0; i < fakeIPStoreMaxEntries+8; i++ {
		store.Create(fmt.Sprintf("filler-%d.example.com.", i))
	}

	if domain, ok := store.Lookup(first); ok && domain == "victim.example.com." {
		t.Fatalf("evicted domain still resolvable via addressCache: %s -> %s", first, domain)
	}
}

// IP 回绕复用时，旧域名的正向映射必须失效，避免反查出错误域名。
func TestFakeIPStoreRebindsAddressAfterWrap(t *testing.T) {
	store := NewFakeIPStore()

	store.mu.Lock()
	store.current4 = store.last4.Prev()
	store.mu.Unlock()

	wrapped, _ := store.Create("wrapped.example.com.")
	next, _ := store.Create("after-wrap.example.com.")

	if wrapped == next {
		t.Fatal("expected address wrap to hand out a different address than the last one")
	}
	if domain, ok := store.Lookup(wrapped); !ok || domain != "wrapped.example.com." {
		t.Fatalf("Lookup(%s) = %q, %v; want wrapped.example.com., true", wrapped, domain, ok)
	}
	if domain, ok := store.Lookup(next); !ok || domain != "after-wrap.example.com." {
		t.Fatalf("Lookup(%s) = %q, %v; want after-wrap.example.com., true", next, domain, ok)
	}
}
