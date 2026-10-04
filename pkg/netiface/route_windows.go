//go:build windows

package netiface

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// defaultRouteMetrics maps interface index to the effective metric of the
// default route (0.0.0.0/0 or ::/0) installed on it.
//
// The route metric and the interface metric are separate in Windows and are
// summed, so both are read: a route with a low metric on a high-metric adapter
// can still lose to the reverse pairing.
func defaultRouteMetrics(family int) map[int]uint32 {
	out := make(map[int]uint32)

	af := uint16(windows.AF_INET)
	if family == FamilyIPv6 {
		af = windows.AF_INET6
	}

	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(af, &table); err != nil || table == nil {
		return out
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	ifaceMetric := make(map[int]uint32)
	disableDefault := make(map[int]bool)
	for _, iface := range systemViews() {
		row := windows.MibIpInterfaceRow{
			Family:         af,
			InterfaceIndex: uint32(iface.Index),
		}
		if err := windows.GetIpInterfaceEntry(&row); err != nil {
			continue
		}
		ifaceMetric[iface.Index] = row.Metric
		disableDefault[iface.Index] = row.DisableDefaultRoutes != 0
	}

	for _, row := range table.Rows() {
		// Only default routes matter; prefix length 0 means "all destinations".
		if row.DestinationPrefix.PrefixLength != 0 {
			continue
		}
		index := int(row.InterfaceIndex)
		metric := row.Metric + ifaceMetric[index]
		if disableDefault[index] {
			// An adapter with default routes disabled is a black hole; make it
			// rank last by inflating its metric past every real candidate.
			metric += 1 << 20
		}
		if current, ok := out[index]; !ok || metric < current {
			out[index] = metric
		}
	}
	return out
}
