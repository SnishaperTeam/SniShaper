//go:build windows

package common

import (
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
	return filepath.Join(execDir, "cert")
}

// ConfigProxyMarker resolves the managed system proxy marker path.
func ConfigProxyMarker(execDir string) string {
	return filepath.Join(execDir, "config", "system_proxy_owner.json")
}
