//go:build linux

package netiface

import (
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

// defaultRouteMetrics reads default routes from netlink and maps each owning
// interface index to its route priority.
//
// A route whose destination is nil is a default route on Linux; the scope and
// protocol are intentionally not filtered, because a default route installed
// by NetworkManager or by the user is exactly the one traffic should follow.
func defaultRouteMetrics(family int) map[int]uint32 {
	out := make(map[int]uint32)

	routes, err := netlink.RouteList(nil, familyLinkFamily(family))
	if err != nil {
		return out
	}
	for _, route := range routes {
		if route.LinkIndex <= 0 || !isDefaultDestination(route.Dst) {
			continue
		}
		metric := uint32(route.Priority)
		if current, ok := out[route.LinkIndex]; !ok || metric < current {
			out[route.LinkIndex] = metric
		}
	}
	return out
}

// isDefaultDestination reports whether a route destination is the default route.
func isDefaultDestination(dst *net.IPNet) bool {
	if dst == nil {
		return true
	}
	ones, _ := dst.Mask.Size()
	return ones == 0
}

func familyLinkFamily(family int) int {
	if family == FamilyIPv6 {
		return netlink.FAMILY_V6
	}
	return netlink.FAMILY_V4
}

// defaultRouteGateways returns the gateway address of each default route, used
// for diagnostics.
func defaultRouteGateways(family int) map[int]netip.Addr {
	out := make(map[int]netip.Addr)

	routes, err := netlink.RouteList(nil, familyLinkFamily(family))
	if err != nil {
		return out
	}
	for _, route := range routes {
		if route.LinkIndex <= 0 || !isDefaultDestination(route.Dst) || route.Gw == nil {
			continue
		}
		if parsed, ok := netip.AddrFromSlice(route.Gw); ok {
			out[route.LinkIndex] = parsed.Unmap()
		}
	}
	return out
}
