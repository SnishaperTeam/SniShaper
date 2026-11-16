//go:build !windows

package app

import "fmt"

// RunAutoStartServiceHost is the Windows service-host entry; off Windows the
// desktop binary is never launched with --service-host, so reaching this is
// a caller error.
func RunAutoStartServiceHost() error {
	return fmt.Errorf("service autostart host is not supported on this platform")
}
