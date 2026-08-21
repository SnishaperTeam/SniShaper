package sysproxy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Linux system proxy is configured through GSettings (org.gnome.system.proxy),
// the standard mechanism on Ubuntu/GNOME and Debian GNOME desktops.
//
// When the app runs elevated (root, via sudo), gsettings writes land in
// root's dconf and are invisible to the user session. Commands are therefore
// re-dispatched to $SUDO_USER preserving the user's session bus so the users'
// own desktop proxy settings change.

const (
	proxySchema = "org.gnome.system.proxy"
	httpSchema  = "org.gnome.system.proxy.http"
	httpsSchema = "org.gnome.system.proxy.https"

	proxyHostLinux   = "127.0.0.1"
	proxyOverrideGDN = "['localhost', '127.0.0.0/8', '::1']"
)

var (
	// Cache for system proxy status
	cachedStatus SystemProxyStatus
	lastCheck    time.Time
	cacheMu      sync.Mutex
)

type SystemProxyStatus struct {
	Enabled  bool
	Server   string
	Override string
}

// runGSettings executes a gsettings command, routed through the invoking user
// when running as root (SUDO_USER). Returns combined stdout.
func runGSettings(args ...string) ([]byte, error) {
	bin, err := exec.LookPath("gsettings")
	if err != nil {
		return nil, fmt.Errorf("gsettings not found (GNOME desktop required): %w", err)
	}
	var cmd *exec.Cmd
	if os.Geteuid() == 0 {
		user := os.Getenv("SUDO_USER")
		if user != "" {
			envs := []string{"DBUS_SESSION_BUS_ADDRESS", "XDG_RUNTIME_DIR", "DISPLAY", "WAYLAND_DISPLAY", "XDG_CURRENT_DESKTOP"}
			full := make([]string, 0, 5+len(args))
			full = append(full, "-u", user, "--preserve-env="+strings.Join(envs, ","), bin)
			full = append(full, args...)
			cmd = exec.Command("sudo", full...)
		} else {
			cmd = exec.Command(bin, args...)
		}
	} else {
		cmd = exec.Command(bin, args...)
	}
	return cmd.Output()
}

func gsettingsGet(schema, key string) (string, error) {
	out, err := runGSettings("get", schema, key)
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(out))
	v = strings.Trim(v, "'")
	return v, nil
}

func gsettingsSet(schema, key, value string) error {
	_, err := runGSettings("set", schema, key, value)
	return err
}

func GetSystemProxyStatus() SystemProxyStatus {
	cacheMu.Lock()
	if !lastCheck.IsZero() && time.Since(lastCheck) < 2*time.Second {
		status := cachedStatus
		cacheMu.Unlock()
		return status
	}
	cacheMu.Unlock()

	status := SystemProxyStatus{}
	if mode, err := gsettingsGet(proxySchema, "mode"); err == nil && strings.EqualFold(mode, "manual") {
		status.Enabled = true
		host, _ := gsettingsGet(httpSchema, "host")
		port, _ := gsettingsGet(httpSchema, "port")
		p, _ := strconv.Atoi(strings.TrimSpace(port))
		if strings.TrimSpace(host) != "" && p > 0 {
			status.Server = net.JoinHostPort(strings.TrimSpace(host), strconv.Itoa(p))
		}
		if ignore, err := gsettingsGet(proxySchema, "ignore-hosts"); err == nil {
			status.Override = strings.TrimRight(strings.TrimLeft(ignore, "["), "]")
		}
	}

	cacheMu.Lock()
	cachedStatus = status
	lastCheck = time.Now()
	cacheMu.Unlock()

	return status
}

func SetSystemProxy(enable bool, server string) error {
	if enable {
		mode, err := gsettingsGet(proxySchema, "mode")
		if err != nil {
			return fmt.Errorf("[sysproxy] gsettings not available: %w", err)
		}
		if strings.EqualFold(strings.TrimSpace(mode), "auto") {
			// 'auto' mode uses a PAC URL and cannot be combined with manual
			// host/port values; switch to manual explicitly.
			return fmt.Errorf("[sysproxy] GNOME proxy is in 'auto' (PAC) mode; switch to manual or none in GNOME network settings first")
		}

		host, portStr, err := net.SplitHostPort(strings.TrimSpace(server))
		if err != nil {
			return fmt.Errorf("[sysproxy] invalid server %q: %w", server, err)
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("[sysproxy] invalid port in %q", server)
		}

		if err := gsettingsSet(proxySchema, "mode", "'manual'"); err != nil {
			return fmt.Errorf("[sysproxy] set mode manual: %w", err)
		}
		for _, schema := range []string{httpSchema, httpsSchema} {
			if err := gsettingsSet(schema, "host", "'"+host+"'"); err != nil {
				return fmt.Errorf("[sysproxy] set %s host: %w", schema, err)
			}
			if err := gsettingsSet(schema, "port", strconv.Itoa(port)); err != nil {
				return fmt.Errorf("[sysproxy] set %s port: %w", schema, err)
			}
		}
		if err := gsettingsSet(proxySchema, "ignore-hosts", proxyOverrideGDN); err != nil {
			return fmt.Errorf("[sysproxy] set ignore-hosts: %w", err)
		}
	} else {
		if err := gsettingsSet(proxySchema, "mode", "'none'"); err != nil {
			return fmt.Errorf("[sysproxy] set mode none: %w", err)
		}
	}

	cacheMu.Lock()
	lastCheck = time.Time{}
	cacheMu.Unlock()

	return nil
}

func EnableSystemProxy(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("[sysproxy] invalid port: %d", port)
	}
	return SetSystemProxy(true, fmt.Sprintf("%s:%d", proxyHostLinux, port))
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

// SetSystemProxyManual opens the desktop network settings UI.
func SetSystemProxyManual() error {
	for _, bin := range []string{"gnome-control-center", "nm-connection-editor"} {
		if path, err := exec.LookPath(bin); err == nil {
			cmd := exec.Command(path, "network")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return cmd.Start()
		}
	}
	return fmt.Errorf("no desktop network settings tool found (gnome-control-center / nm-connection-editor)")
}