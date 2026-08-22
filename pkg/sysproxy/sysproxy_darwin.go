//go:build darwin

package sysproxy

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const proxyHostDarwin = "127.0.0.1"

type SystemProxyStatus struct {
	Enabled  bool
	Server   string
	Override string
}

var (
	cachedStatusDarwin SystemProxyStatus
	lastCheckDarwin    time.Time
	cacheMuDarwin      sync.Mutex
)

func darwinServices() ([]string, error) {
	out, err := exec.Command("networksetup", "-listallnetworkservices").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("[sysproxy] networksetup not available: %w", err)
	}
	var services []string
	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "*") || strings.Contains(name, "An asterisk") {
			continue
		}
		services = append(services, name)
	}
	return services, nil
}

func darwinGetProxy(kind string) (enabled bool, host string, port int) {
	services, err := darwinServices()
	if err != nil {
		return false, "", 0
	}
	for _, svc := range services {
		out, err := exec.Command("networksetup", "-get"+kind, svc).CombinedOutput()
		if err != nil {
			continue
		}
		lines := strings.Split(string(out), "\n")
		var h, p string
		var en bool
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Enabled:") {
				en = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "Enabled:")), "yes")
			} else if strings.HasPrefix(line, "Server:") {
				h = strings.TrimSpace(strings.TrimPrefix(line, "Server:"))
			} else if strings.HasPrefix(line, "Port:") {
				p = strings.TrimSpace(strings.TrimPrefix(line, "Port:"))
			}
		}
		if en && h != "" {
			port, _ := strconv.Atoi(p)
			return true, h, port
		}
	}
	return false, "", 0
}

func GetSystemProxyStatus() SystemProxyStatus {
	cacheMuDarwin.Lock()
	if !lastCheckDarwin.IsZero() && time.Since(lastCheckDarwin) < 2*time.Second {
		status := cachedStatusDarwin
		cacheMuDarwin.Unlock()
		return status
	}
	cacheMuDarwin.Unlock()

	status := SystemProxyStatus{}
	if en, host, port := darwinGetProxy("webproxy"); en {
		status.Enabled = true
		status.Server = net.JoinHostPort(host, strconv.Itoa(port))
	}
	if !status.Enabled {
		if en, host, port := darwinGetProxy("socksfirewallproxy"); en {
			status.Enabled = true
			status.Server = net.JoinHostPort(host, strconv.Itoa(port))
		}
	}

	cacheMuDarwin.Lock()
	cachedStatusDarwin = status
	lastCheckDarwin = time.Now()
	cacheMuDarwin.Unlock()

	return status
}

func darwinSetProxy(kind string, enable bool, host string, port int) error {
	services, err := darwinServices()
	if err != nil {
		return err
	}
	for _, svc := range services {
		if enable {
			if err := exec.Command("networksetup", "-set"+kind, svc, host, strconv.Itoa(port)).Run(); err != nil {
				return fmt.Errorf("[sysproxy] set %s on %q: %w", kind, svc, err)
			}
		} else {
			if err := exec.Command("networksetup", "-set"+kind+"state", svc, "off").Run(); err != nil {
				return fmt.Errorf("[sysproxy] disable %s on %q: %w", kind, svc, err)
			}
		}
	}
	return nil
}

func SetSystemProxy(enable bool, server string) error {
	if enable {
		host, portStr, err := net.SplitHostPort(strings.TrimSpace(server))
		if err != nil {
			return fmt.Errorf("[sysproxy] invalid server %q: %w", server, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("[sysproxy] invalid port in %q", server)
		}
		if err := darwinSetProxy("webproxy", true, host, port); err != nil {
			return err
		}
		if err := darwinSetProxy("securewebproxy", true, host, port); err != nil {
			return err
		}
	} else {
		if err := darwinSetProxy("webproxy", false, "", 0); err != nil {
			return err
		}
		if err := darwinSetProxy("securewebproxy", false, "", 0); err != nil {
			return err
		}
	}

	cacheMuDarwin.Lock()
	lastCheckDarwin = time.Time{}
	cacheMuDarwin.Unlock()

	return nil
}

func EnableSystemProxy(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("[sysproxy] invalid port: %d", port)
	}
	return SetSystemProxy(true, fmt.Sprintf("%s:%d", proxyHostDarwin, port))
}

func DisableSystemProxy() error {
	return SetSystemProxy(false, "")
}

func GetSystemProxyStatusSafe() (SystemProxyStatus, error) {
	return GetSystemProxyStatus(), nil
}

var originalProxySettings *SystemProxyStatus

func SaveOriginalProxySettings() error {
	status := GetSystemProxyStatus()
	originalProxySettings = &status
	return nil
}

func SetOriginalProxySettings(status SystemProxyStatus) {
	copy := status
	originalProxySettings = &copy
}

func RestoreOriginalProxySettings() error {
	if originalProxySettings == nil {
		return nil
	}
	if originalProxySettings.Enabled {
		return SetSystemProxy(true, originalProxySettings.Server)
	}
	return SetSystemProxy(false, "")
}

// SetSystemProxyManual opens the macOS network settings UI.
func SetSystemProxyManual() error {
	return exec.Command("open", "x-apple.systempreferences:com.apple.systempreferences.Network").Run()
}
