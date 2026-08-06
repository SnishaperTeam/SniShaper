package app

import (
	"fmt"
	"net"
	"time"
)

// checkIPv6Available reports whether any up interface carries a global
// unicast IPv6 address. Link-local (fe80::/10) and ULA (fd00::/8) addresses
// are excluded — they exist even on pure-IPv4 networks and cannot reach the
// IPv6 internet.
func (a *App) checkIPv6Available() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			if ip.To4() == nil && ip.IsGlobalUnicast() && !ip.IsPrivate() {
				return true
			}
		}
	}
	return false
}

// ensureIPv6 is the backend backstop for IPv6-dependent operations: it
// rejects them on IPv4-only networks even if a caller bypasses the UI.
func (a *App) ensureIPv6() error {
	if a.checkIPv6Available() {
		return nil
	}
	return fmt.Errorf("当前网络为纯 IPv4，IPv6 功能不可用 (IPv4-only network, IPv6 features unavailable)")
}

// GetIPv6Available returns whether the current network has usable IPv6.
func (a *App) GetIPv6Available() bool {
	return a.checkIPv6Available()
}

// RefreshIPv6Check re-checks IPv6 availability, notifies the frontend and
// returns the current result.
func (a *App) RefreshIPv6Check() bool {
	available := a.checkIPv6Available()
	a.appendLog(fmt.Sprintf("[network] IPv6 availability check: %v", available))
	a.emitFrontendState()
	return available
}

// startIPv6Monitor re-checks IPv6 availability every 10 minutes while the
// app runs. Runs until shutdown; use RefreshIPv6Check from Settings for an
// immediate re-check.
func (a *App) startIPv6Monitor() {
	a.runSafeAsync("ipv6 monitor", func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.RefreshIPv6Check()
			case <-a.ctx.Done():
				return
			}
		}
	})
}
