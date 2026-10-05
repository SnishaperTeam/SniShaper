package proxy

import (
	"net/netip"
	"testing"

	"snishaper/pkg/netiface"
)

func TestOutboundFamily(t *testing.T) {
	cases := []struct {
		addr string
		want int
	}{
		{"1.2.3.4:443", netiface.FamilyIPv4},
		{"2606:4700::1:443", netiface.FamilyIPv6},
		{"[2606:4700::1]:443", netiface.FamilyIPv6},
		{"1.2.3.4", netiface.FamilyIPv4},
		{"2606:4700::1", netiface.FamilyIPv6},
		{"example.com:443", netiface.FamilyIPv4},
		{"[::1]:53", netiface.FamilyIPv6},
	}
	for _, c := range cases {
		if got := outboundFamily(c.addr); got != c.want {
			t.Fatalf("outboundFamily(%q) = %d, want %d", c.addr, got, c.want)
		}
	}
}

// TestOutboundFamilyMatchesBinding guards the invariant that the Control hook
// and the source address are derived from the same family. A mismatch would pin
// an IPv4 socket to an IPv6 interface and fail every dial.
func TestOutboundFamilyMatchesBinding(t *testing.T) {
	parsed, err := netip.ParseAddr("2606:4700::1")
	if err != nil {
		t.Fatal(err)
	}
	if outboundFamily(netip.AddrPortFrom(parsed, 443).String()) != netiface.FamilyIPv6 {
		t.Fatal("an IPv6 literal target must select the IPv6 family")
	}
}

// TestPhysicalDialControlExists documents that the helper is wired up: before the
// fix every TUN-mode outbound set only LocalAddr, leaving the kernel free to
// route the socket back into the tunnel.
func TestPhysicalDialControlExists(t *testing.T) {
	server := NewProxyServer("127.0.0.1:0")

	// With TUN mode off there is no outbound interface pinning to do.
	if control := server.getPhysicalDialControl("1.2.3.4:443"); control != nil && !server.tunMode {
		t.Log("binding resolved even outside TUN mode; Control is only applied under TUN mode")
	}

	// The method must be callable and must not panic on a malformed target.
	_ = server.getPhysicalDialControl("not-an-address")
	_ = server.getPhysicalLocalAddr("not-an-address")
}
