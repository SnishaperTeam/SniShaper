//go:build linux

package common

import (
	"path/filepath"
)

// ConfigSettingsPath resolves the settings file path for the current platform.
func ConfigSettingsPath(execDir string) string {
	p, _ := EnsureUserConfig(execDir, filepath.Join("config", "settings.json"))
	return p
}

// ConfigRulesPath resolves the rules config file path for the current platform.
func ConfigRulesPath(execDir string) string {
	p, _ := EnsureUserConfig(execDir, filepath.Join("rules", "config.json"))
	return p
}

// ConfigCertDir resolves the certificate directory for the current platform.
func ConfigCertDir(_ string) string {
	return UserConfigPath("cert")
}

// ConfigProxyMarker resolves the managed system proxy marker path.
func ConfigProxyMarker(_ string) string {
	return UserConfigPath(filepath.Join("config", "system_proxy_owner.json"))
}
