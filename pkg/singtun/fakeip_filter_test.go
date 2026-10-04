package singtun

import (
	"net/netip"
	"testing"
)

// TestShouldBypassFakeIP covers the Windows connectivity probe, which is the
// reason this list exists: a fake-ip answer makes NCSI mark the adapter as
// having no Internet access.
func TestShouldBypassFakeIP(t *testing.T) {
	bypass := []string{
		"dns.msftncsi.com",
		"www.msftconnecttest.com",
		"www.msftncsi.com",
		"msftncsi.com",
		"DNS.MSFtncsi.COM",
		"www.msftconnecttest.com.",
		"dns.msftncsi.com.",
		"stun.l.google.com",
		"a.b.msftncsi.com",
	}
	for _, name := range bypass {
		if !shouldBypassFakeIP(name) {
			t.Errorf("shouldBypassFakeIP(%q) = false, want true", name)
		}
	}

	fake := []string{
		"example.com",
		"www.bing.com",
		"microsoft.com",
		"cdn.gh-proxy.org",
		"",
		"notmsftncsi.com",
		"msftncsi.com.cn",
	}
	for _, name := range fake {
		if shouldBypassFakeIP(name) {
			t.Errorf("shouldBypassFakeIP(%q) = true, want false", name)
		}
	}
}

// TestIsNCSIProbe pins the set of names answered locally rather than resolved.
func TestIsNCSIProbe(t *testing.T) {
	local := []string{
		"dns.msftncsi.com",
		"DNS.MSFtncsi.COM",
		"www.msftconnecttest.com",
		"www.msftconnecttest.com.",
		"www.msftncsi.com",
		"msftncsi.com",
		"msftconnecttest.com",
	}
	for _, name := range local {
		if !isNCSIProbe(name) {
			t.Errorf("isNCSIProbe(%q) = false, want true", name)
		}
	}

	upstream := []string{
		"stun.l.google.com",
		"example.com",
		"microsoft.com",
		"",
	}
	for _, name := range upstream {
		if isNCSIProbe(name) {
			t.Errorf("isNCSIProbe(%q) = true, want false", name)
		}
	}
}

// TestNCSIStaticIPIsPublic guards the invariant that makes the local answer
// work: Windows rejects the answer unless it is a public address.
func TestNCSIStaticIPIsPublic(t *testing.T) {
	addr := netip.MustParseAddr(ncsiStaticIP)
	if !addr.Is4() {
		t.Fatalf("ncsiStaticIP = %s, want IPv4", addr)
	}
	if netip.MustParsePrefix("198.18.0.0/15").Contains(addr) {
		t.Fatalf("ncsiStaticIP = %s must not be inside the fake-ip range", addr)
	}
	if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
		t.Fatalf("ncsiStaticIP = %s is not a public address", addr)
	}
}

// TestLookupNCSIHost pins the reverse mapping that lets the proxy recognise
// the probe: Windows connects to the address DNS returned, so without this the
// proxy would only see a bare IP and would not match the NCSI host.
func TestLookupNCSIHost(t *testing.T) {
	domain, ok := lookupNCSIHost(netip.MustParseAddr(ncsiStaticIP))
	if !ok {
		t.Fatal("lookupNCSIHost(ncsiStaticIP) not found")
	}
	if domain != "www.msftconnecttest.com" {
		t.Fatalf("lookupNCSIHost = %q, want www.msftconnecttest.com", domain)
	}
	if _, ok := lookupNCSIHost(netip.MustParseAddr("1.1.1.1")); ok {
		t.Fatal("lookupNCSIHost(1.1.1.1) = found, want not found")
	}
	if _, ok := lookupNCSIHost(netip.Addr{}); ok {
		t.Fatal("lookupNCSIHost(invalid) = found, want not found")
	}
}
