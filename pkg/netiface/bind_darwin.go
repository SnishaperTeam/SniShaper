//go:build darwin

package netiface

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// bindInterface pins a socket to an interface index using the BSD bound-if
// options. Unlike Linux there is no SO_BINDTOIFINDEX, so the index is passed
// directly to IP_BOUND_IF / IPV6_BOUND_IF at the IP layer.
func bindInterface(fd uintptr, index int) error {
	if index <= 0 {
		return fmt.Errorf("invalid interface index %d", index)
	}
	if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IP, unix.IP_BOUND_IF, index); err != nil {
		return fmt.Errorf("bind IPv4 interface %d: %w", index, err)
	}
	if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_IPV6, unix.IPV6_BOUND_IF, index); err != nil {
		return fmt.Errorf("bind IPv6 interface %d: %w", index, err)
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
			sockErr = bindInterface(fd, index)
		})
		if err != nil {
			return err
		}
		return sockErr
	}
}
