//go:build !windows && !linux && !darwin

package netiface

import (
	"fmt"
	"syscall"
)

// bindInterface has no portable per-interface socket option on this platform.
func bindInterface(fd uintptr, index int) error {
	return fmt.Errorf("per-interface socket binding is unsupported on this platform")
}

// Control returns a no-op hook so callers need no build tags.
func (b Binding) Control(family int) func(network, address string, c syscall.RawConn) error {
	return nil
}
