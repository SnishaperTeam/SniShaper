package sysproxy

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows/registry"
)

const (
	proxySettingsKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	internetOptionRefresh         = 37
	internetOptionSettingsChanged = 39
)

var (
	wininetDLL             = syscall.NewLazyDLL("wininet.dll")
	internetSetOptionProc  = wininetDLL.NewProc("InternetSetOptionW")
	
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

func notifyProxyChange() error {
	if err := callInternetSetOption(internetOptionSettingsChanged); err != nil {
		return err
	}
	return callInternetSetOption(internetOptionRefresh)
}

func callInternetSetOption(option uintptr) error {
	ret, _, callErr := internetSetOptionProc.Call(0, option, 0, 0)
	if ret != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return fmt.Errorf("InternetSetOptionW failed for option %d", option)
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
	k, err := registry.OpenKey(registry.CURRENT_USER, proxySettingsKey, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()

		enableVal, _, err := k.GetIntegerValue("ProxyEnable")
		if err == nil && enableVal == 1 {
			status.Enabled = true
		}

		server, _, err := k.GetStringValue("ProxyServer")
		if err == nil {
			status.Server = strings.TrimSpace(server)
		}

		override, _, err := k.GetStringValue("ProxyOverride")
		if err == nil {
			status.Override = strings.TrimSpace(override)
		}
	}

	cacheMu.Lock()
	cachedStatus = status
	lastCheck = time.Now()
	cacheMu.Unlock()

	return status
}

func SetSystemProxy(enable bool, server string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, proxySettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("[sysproxy] failed to open registry key: %w", err)
	}
	defer k.Close()

	enableVal := uint64(0)
	if enable {
		enableVal = 1
	}
	if err := k.SetDWordValue("ProxyEnable", uint32(enableVal)); err != nil {
		return fmt.Errorf("[sysproxy] failed to set ProxyEnable: %w", err)
	}

	if enable {
		if server == "" {
			return fmt.Errorf("[sysproxy] server cannot be empty when enabling proxy")
		}
		if err := k.SetStringValue("ProxyServer", server); err != nil {
			return fmt.Errorf("[sysproxy] failed to set ProxyServer: %w", err)
		}
		override := "<local>"
		if err := k.SetStringValue("ProxyOverride", override); err != nil {
			return fmt.Errorf("[sysproxy] failed to set ProxyOverride: %w", err)
		}
	}

	cacheMu.Lock()
	lastCheck = time.Time{}
	cacheMu.Unlock()

	return notifyProxyChange()
}

func EnableSystemProxy(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("[sysproxy] invalid port: %d", port)
	}
	server := fmt.Sprintf("127.0.0.1:%d", port)
	return SetSystemProxy(true, server)
}

func DisableSystemProxy() error {
	return SetSystemProxy(false, "")
}

func GetSystemProxyStatusSafe() (SystemProxyStatus, error) {
	status := GetSystemProxyStatus()
	return status, nil
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

	k, err := registry.OpenKey(registry.CURRENT_USER, proxySettingsKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("[sysproxy] failed to open registry key for restore: %w", err)
	}
	defer k.Close()

	enableVal := uint64(0)
	if originalProxySettings.Enabled {
		enableVal = 1
	}
	_ = k.SetDWordValue("ProxyEnable", uint32(enableVal))

	if originalProxySettings.Enabled && originalProxySettings.Server != "" {
		_ = k.SetStringValue("ProxyServer", originalProxySettings.Server)
		if originalProxySettings.Override != "" {
			_ = k.SetStringValue("ProxyOverride", originalProxySettings.Override)
		}
	}

	cacheMu.Lock()
	lastCheck = time.Time{}
	cacheMu.Unlock()

	return notifyProxyChange()
}

// SetSystemProxyManual opens the Windows proxy settings UI
func SetSystemProxyManual() error {
	return startHiddenCommand("cmd", "/c", "start", "ms-settings:network-proxy")
}
