//go:build windows

package netiface

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
)

// Windows socket option levels and names for per-interface binding. They are
// not exported by x/sys/windows, and the IPv4/IPv6 pair intentionally differs
// in both level and byte order (see bindInterface).
const (
	ipUnicastIf   = 31
	ipv6UnicastIf = 31
)

// htonl converts a host-order 32-bit value to network byte order. Windows'
// IP_UNICAST_IF expects network byte order while IPV6_UNICAST_IF expects host
// byte order, so both forms are needed.
func htonl(v uint32) uint32 {
	return ((v & 0xff) << 24) | ((v & 0xff00) << 8) | ((v >> 8) & 0xff00) | ((v >> 24) & 0xff)
}

// bindInterface pins a socket to the interface owning index.
//
// The two families are not symmetric and getting either detail wrong fails
// late and opaquely, so both are spelled out here:
//
//	IP_UNICAST_IF  at level IPPROTO_IP    takes the index in NETWORK byte order
//	IPV6_UNICAST_IF at level IPPROTO_IPV6 takes the index in HOST byte order
//
// Using the wrong order yields WSAEADDRNOTAVAIL ("The requested address is not
// valid in its context") for IPv4 and WSAEINVAL for IPv6. Both were confirmed
// against a live Windows host.
func bindInterface(fd windows.Handle, index int, family int) error {
	if index <= 0 {
		return fmt.Errorf("invalid interface index %d", index)
	}
	value := int(index)
	level := int(syscall.IPPROTO_IP)
	option := ipUnicastIf

	if family == FamilyIPv6 {
		level = int(windows.IPPROTO_IPV6)
		option = ipv6UnicastIf
		value = index
	} else {
		value = int(htonl(uint32(index)))
	}

	if err := windows.SetsockoptInt(fd, level, option, value); err != nil {
		return fmt.Errorf("bind interface %d (family %d): %w", index, family, err)
	}
	return nil
}

// Control returns a net.Dialer Control hook that binds the socket to b.
func (b Binding) Control(family int) func(network, address string, c syscall.RawConn) error {
	if b.InterfaceIndex <= 0 {
		return nil
	}
	index := b.InterfaceIndex
	return func(network, address string, c syscall.RawConn) error {
		var sockErr error
		err := c.Control(func(fd uintptr) {
			sockErr = bindInterface(windows.Handle(fd), index, family)
		})
		if err != nil {
			return err
		}
		return sockErr
	}
}
