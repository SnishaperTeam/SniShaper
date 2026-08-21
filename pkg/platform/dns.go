//go:build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const resolvConfPath = "/etc/resolv.conf"

// SetSystemDNS points the system at the given nameservers and returns a
// restore function.
//
// When /etc/resolv.conf is owned by a dynamic resolver (systemd-resolved,
// NetworkManager, resolvconf — detected via symlink target), the file is left
// untouched and a no-op restore is returned: those managers own DNS and the
// TUN layer drives nameservers through resolvectl instead.
//
// When resolv.conf is a plain static file (e.g. bare Debian), the original
// content is backed up, rewritten with the given nameservers, and restore
// rewinds it. Callers must invoke restore when the tunnel/proxy shuts down.
func SetSystemDNS(nameservers []string) (restore func() error, err error) {
	if len(nameservers) == 0 {
		return noopRestore, fmt.Errorf("no nameservers given")
	}
	if !isStaticResolvConf() {
		return noopRestore, nil
	}

	original, err := os.ReadFile(resolvConfPath)
	if err != nil {
		return noopRestore, fmt.Errorf("read %s: %w", resolvConfPath, err)
	}

	var b strings.Builder
	b.WriteString("# managed by SniShaper (TUN DNS)\n")
	for _, ns := range nameservers {
		b.WriteString("nameserver ")
		b.WriteString(ns)
		b.WriteString("\n")
	}

	if err := os.WriteFile(resolvConfPath, []byte(b.String()), 0644); err != nil {
		return noopRestore, fmt.Errorf("write %s: %w", resolvConfPath, err)
	}

	restore = func() error {
		if len(original) == 0 {
			return os.Remove(resolvConfPath)
		}
		return os.WriteFile(resolvConfPath, original, 0644)
	}
	return restore, nil
}

func isStaticResolvConf() bool {
	target, err := filepath.EvalSymlinks(resolvConfPath)
	if err != nil {
		// No resolv.conf at all; treat as a plain file we can create.
		return true
	}
	return !isManagedResolverTarget(target)
}

// isManagedResolverTarget reports whether a resolv.conf symlink target belongs
// to a dynamic resolver (systemd-resolved, resolvconf, NetworkManager) that
// owns DNS configuration.
func isManagedResolverTarget(target string) bool {
	managed := []string{"systemd/resolve", "resolvconf", "NetworkManager"}
	for _, m := range managed {
		if strings.Contains(target, m) {
			return true
		}
	}
	return false
}

func noopRestore() error { return nil }