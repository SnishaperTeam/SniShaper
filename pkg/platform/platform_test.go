//go:build linux

package platform

import "testing"

// Self-check for the pure DNS/logic helpers. The syscall- and netlink-backed
// primitives (OpenTun, routes, SetSystemDNS write path) require root and a
// running tunnel, so they are exercised at runtime on the target host rather
// than in unit tests.
func TestManagedResolverTarget(t *testing.T) {
	cases := []struct {
		target string
		want   bool
	}{
		{"/run/systemd/resolve/stub-resolv.conf", true},
		{"/run/systemd/resolve/resolv.conf", true},
		{"/etc/resolvconf/run/resolv.conf", true},
		{"/run/NetworkManager/resolv.conf", true},
		{"/etc/resolv.conf", false},
		{"/var/run/foo.conf", false},
	}
	for _, tc := range cases {
		if got := isManagedResolverTarget(tc.target); got != tc.want {
			t.Errorf("isManagedResolverTarget(%q) = %v, want %v", tc.target, got, tc.want)
		}
	}
}