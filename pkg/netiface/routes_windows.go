//go:build windows

package netiface

import (
	"net"
	"net/netip"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsIfaceMeta struct {
	metric         uint32
	disableDefault bool
}

func windowsAddressFamily(family int) uint16 {
	if family == FamilyIPv6 {
		return windows.AF_INET6
	}
	return windows.AF_INET
}

func lookupDefaultRoutes(family int) []defaultRoute {
	af := windowsAddressFamily(family)
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(af, &table); err != nil {
		return nil
	}
	if table == nil {
		return nil
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	meta := windowsInterfaceMeta(af)
	routes := make([]defaultRoute, 0, 4)
	for _, row := range table.Rows() {
		if row.DestinationPrefix.PrefixLength != 0 {
			continue
		}
		route := defaultRoute{
			interfaceIndex: int(row.InterfaceIndex),
			gateway:        windowsNextHop(row.NextHop, af),
			metric:         row.Metric,
			hasMetric:      true,
		}
		if info, ok := meta[route.interfaceIndex]; ok {
			route.metric += info.metric
			route.disableDefault = info.disableDefault
		}
		routes = append(routes, route)
	}
	return routes
}

func windowsNextHop(nextHop windows.RawSockaddrInet, af uint16) netip.Addr {
	if uint16(nextHop.Family) != af {
		return netip.Addr{}
	}
	if af == windows.AF_INET {
		sockaddr := (*windows.RawSockaddrInet4)(unsafe.Pointer(&nextHop))
		return netip.AddrFrom4(sockaddr.Addr)
	}
	sockaddr := (*windows.RawSockaddrInet6)(unsafe.Pointer(&nextHop))
	addr := netip.AddrFrom16(sockaddr.Addr)
	if addr.IsLinkLocalUnicast() && sockaddr.Scope_id != 0 {
		return addr.WithZone(strconv.FormatUint(uint64(sockaddr.Scope_id), 10))
	}
	return addr
}

func windowsInterfaceMeta(af uint16) map[int]windowsIfaceMeta {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	out := make(map[int]windowsIfaceMeta, len(interfaces))
	for _, iface := range interfaces {
		row := windows.MibIpInterfaceRow{
			Family:         af,
			InterfaceIndex: uint32(iface.Index),
		}
		if err := windows.GetIpInterfaceEntry(&row); err != nil {
			continue
		}
		out[iface.Index] = windowsIfaceMeta{
			metric:         row.Metric,
			disableDefault: row.DisableDefaultRoutes != 0,
		}
	}
	return out
}
