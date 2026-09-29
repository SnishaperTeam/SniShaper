//go:build darwin

package netiface

import (
	"net/netip"
	"strconv"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

func lookupDefaultRoutes(family int) []defaultRoute {
	rib, err := route.FetchRIB(unix.AF_UNSPEC, route.RIBTypeRoute, 0)
	if err != nil {
		return nil
	}
	messages, err := route.ParseRIB(route.RIBTypeRoute, rib)
	if err != nil {
		return nil
	}

	wantV6 := family == FamilyIPv6
	out := make([]defaultRoute, 0, 4)
	for _, message := range messages {
		routeMessage, ok := message.(*route.RouteMessage)
		if !ok || routeMessage.Index <= 0 {
			continue
		}
		if !isDefaultRouteDestination(routeMessage.Addrs) {
			continue
		}
		gateway := gatewayFromAddrs(routeMessage.Addrs, wantV6, routeMessage.Flags&unix.RTF_GATEWAY != 0)
		if gateway.IsValid() {
			if gateway.Is4() == wantV6 {
				continue
			}
		} else if addressFamilyOfAddrs(routeMessage.Addrs) != addressFamily(wantV6) {
			continue
		}
		out = append(out, defaultRoute{
			interfaceIndex: routeMessage.Index,
			gateway:        gateway,
		})
	}
	return out
}

func addressFamily(wantV6 bool) int {
	if wantV6 {
		return unix.AF_INET6
	}
	return unix.AF_INET
}

func addressFamilyOfAddrs(addrs []route.Addr) int {
	for _, addr := range addrs {
		if addr == nil {
			continue
		}
		switch addr.(type) {
		case *route.Inet6Addr:
			return unix.AF_INET6
		case *route.Inet4Addr:
			return unix.AF_INET
		}
	}
	return 0
}

func isDefaultRouteDestination(addrs []route.Addr) bool {
	if len(addrs) == 0 || addrs[0] == nil {
		return true
	}
	switch addr := addrs[0].(type) {
	case *route.DefaultAddr:
		return true
	case *route.Inet4Addr:
		return addr.IP == [4]byte{}
	case *route.Inet6Addr:
		return addr.IP == [16]byte{}
	}
	return false
}

func gatewayFromAddrs(addrs []route.Addr, wantV6 bool, isGateway bool) netip.Addr {
	if !isGateway {
		return netip.Addr{}
	}
	for index := 1; index < len(addrs); index++ {
		switch addr := addrs[index].(type) {
		case *route.Inet6Addr:
			if !wantV6 || addr.IP == [16]byte{} {
				continue
			}
			parsed := netip.AddrFrom16(addr.IP)
			if parsed.IsLinkLocalUnicast() && addr.ZoneID > 0 {
				return parsed.WithZone(strconv.Itoa(addr.ZoneID))
			}
			return parsed
		case *route.Inet4Addr:
			if wantV6 || addr.IP == [4]byte{} {
				continue
			}
			return netip.AddrFrom4(addr.IP)
		}
	}
	return netip.Addr{}
}
