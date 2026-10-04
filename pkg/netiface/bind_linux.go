//go:build linux

package netiface

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/unix"
)

// bindInterface pins a socket to an interface.
//
// SO_BINDTOIFINDEX is preferred because it takes the interface index directly.
// SO_BINDTODEVICE is the fallback for kernels that lack it; it needs the
// interface name and requires CAP_NET_RAW, which the TUN process already has.
func bindInterface(fd uintptr, binding Binding) error {
	if binding.InterfaceIndex > 0 {
		err := unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BINDTOIFINDEX, binding.InterfaceIndex)
		if err == nil {
			return nil
		}
	}
	if binding.InterfaceName == "" {
		return fmt.Errorf("no interface index or name available for binding")
	}
	if err := unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, binding.InterfaceName); err != nil {
		return fmt.Errorf("bind to device %s: %w", binding.InterfaceName, err)
	}
	return nil
}

// Control returns a net.Dialer Control hook that binds the socket to b.
func (b Binding) Control(family int) func(network, address string, c syscall.RawConn) error {
	if b.InterfaceIndex <= 0 && b.InterfaceName == "" {
		return nil
	}
	binding := b
	return func(network, address string, c syscall.RawConn) error {
		var sockErr error
		err := c.Control(func(fd uintptr) {
			sockErr = bindInterface(fd, binding)
		})
		if err != nil {
			return err
		}
		return sockErr
	}
}
