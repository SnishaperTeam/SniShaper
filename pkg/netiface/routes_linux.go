//go:build linux

package netiface

import (
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
)

func lookupDefaultRoutes(family int) []defaultRoute {
	table := netlink.FAMILY_V4
	if family == FamilyIPv6 {
		table = netlink.FAMILY_V6
	}
	routes, err := netlink.RouteList(nil, table)
	if err != nil {
		return nil
	}
	out := make([]defaultRoute, 0, 4)
	for _, route := range routes {
		if route.LinkIndex <= 0 || !isDefaultDst(route.Dst) {
			continue
		}
		metric := uint32(0)
		if route.Priority > 0 {
			metric = uint32(route.Priority)
		}
		out = append(out, defaultRoute{
			interfaceIndex: route.LinkIndex,
			gateway:        unmapIP(route.Gw),
			metric:         metric,
			hasMetric:      true,
		})
	}
	return out
}

func isDefaultDst(dst *net.IPNet) bool {
	if dst == nil {
		return true
	}
	ones, _ := dst.Mask.Size()
	return ones == 0
}

func unmapIP(ip net.IP) netip.Addr {
	if ip == nil {
		return netip.Addr{}
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}
	}
	return addr.Unmap()
}
