//go:build !windows && !linux

package netiface

// defaultRouteMetrics has no portable route-table reader on this platform.
//
// Selection still works: the kernel routing probe in probeSourceAddr asks the
// OS directly which source address outbound traffic would use, which is a
// stronger signal than parsing a routing table would have been.
func defaultRouteMetrics(family int) map[int]uint32 {
	return map[int]uint32{}
}
