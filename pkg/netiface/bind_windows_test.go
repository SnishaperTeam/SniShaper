//go:build windows

package netiface

import (
	"testing"

	"golang.org/x/sys/windows"
)

// TestHtonlSwapsBytesForIPUnicastIf pins the byte order Windows requires for
// IP_UNICAST_IF.
//
// The option is asymmetric on Windows and getting it wrong is silent at
// startup and fatal at connect time: IP_UNICAST_IF takes the interface index
// in network byte order (WSAEADDRNOTAVAIL otherwise), while IPV6_UNICAST_IF
// takes it in host byte order (WSAEINVAL otherwise). Both were confirmed
// against a live Windows host.
func TestHtonlSwapsBytesForIPUnicastIf(t *testing.T) {
	cases := []struct {
		in   uint32
		want uint32
	}{
		{0, 0},
		{1, 1 << 24},
		{27, 27 << 24},
		{256, 0x00010000},
		{0x01020304, 0x04030201},
	}
	for _, tc := range cases {
		if got := htonl(tc.in); got != tc.want {
			t.Errorf("htonl(%d) = %#08x, want %#08x", tc.in, got, tc.want)
		}
	}
	// htonl must be its own inverse, otherwise a round trip corrupts the index.
	for _, value := range []uint32{0, 1, 27, 256, 1 << 20} {
		if got := htonl(htonl(value)); got != value {
			t.Errorf("htonl(htonl(%d)) = %d, want %d", value, got, value)
		}
	}
}

// TestBindInterfaceRejectsBadIndex keeps a zero index from silently unbinding.
func TestBindInterfaceRejectsBadIndex(t *testing.T) {
	if err := bindInterface(windows.Handle(0), 0, FamilyIPv4); err == nil {
		t.Fatal("bindInterface(index=0) = nil, want an error")
	}
	if err := bindInterface(windows.Handle(0), -1, FamilyIPv6); err == nil {
		t.Fatal("bindInterface(index=-1) = nil, want an error")
	}
}
