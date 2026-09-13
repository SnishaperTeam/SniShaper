//go:build windows

package common

import (
	"os"
	"path/filepath"
)

// ConfigSettingsPath resolves the settings file path for the current platform.
func ConfigSettingsPath(execDir string) string {
	return ResolveRuntimeFile(execDir, filepath.Join("config", "settings.json"))
}

// ConfigRulesPath resolves the rules config file path for the current platform.
func ConfigRulesPath(execDir string) string {
	return ResolveRuntimeFile(execDir, filepath.Join("rules", "config.json"))
}

// ConfigCertDir resolves the certificate directory for the current platform.
func ConfigCertDir(execDir string) string {
	stable := UserConfigPath("cert")
	legacy := filepath.Join(execDir, "cert")
	if _, err := os.Stat(stable); os.IsNotExist(err) {
		if _, err2 := os.Stat(legacy); err2 == nil {
			if err := os.MkdirAll(stable, 0755); err == nil {
				if data, err := os.ReadFile(filepath.Join(legacy, "ca.crt")); err == nil {
					_ = os.WriteFile(filepath.Join(stable, "ca.crt"), data, 0644)
				}
				if data, err := os.ReadFile(filepath.Join(legacy, "ca.key")); err == nil {
					_ = os.WriteFile(filepath.Join(stable, "ca.key"), data, 0600)
				}
			}
		}
	}
	return stable
}

// ConfigProxyMarker resolves the managed system proxy marker path.
func ConfigProxyMarker(execDir string) string {
	return filepath.Join(execDir, "config", "system_proxy_owner.json")
}
