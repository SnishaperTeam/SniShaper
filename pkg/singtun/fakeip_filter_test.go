package singtun

import "testing"

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
