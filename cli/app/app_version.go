package app

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildVersion and buildChannel are injected via -ldflags by the CI
// pipeline and build scripts. When empty, the version is derived from
// the Package.appxmanifest next to the executable at runtime, and
// finally falls back to a default. The manifest is the single version
// source for every build, identical to the desktop app.
var (
	buildVersion string
	buildChannel string
)

// normalizeReleaseChannel maps a raw channel token to a canonical value.
func normalizeReleaseChannel(ch string) string {
	v := strings.ToLower(strings.TrimSpace(ch))
	if i := strings.Index(v, "."); i > 0 {
		v = v[:i]
	}
	switch v {
	case "alpha":
		return "alpha"
	case "beta":
		return "beta"
	case "rc", "rc1", "releasecandidate", "release-candidate", "candidate":
		return "rc"
	case "stable", "release", "official", "final":
		return "stable"
	default:
		return "stable"
	}
}

// channelFromTag derives the release channel from a git tag name.
func channelFromTag(tag string) string {
	lower := strings.ToLower(tag)
	switch {
	case strings.Contains(lower, "-alpha"):
		return "alpha"
	case strings.Contains(lower, "-beta"):
		return "beta"
	case strings.Contains(lower, "-rc"):
		return "rc"
	default:
		return "stable"
	}
}

// VersionString returns the effective build version without an App
// instance (used by the CLI entrypoint).
func VersionString() string {
	if strings.TrimSpace(buildVersion) != "" {
		return buildVersion
	}
	if v, err := manifestVersionFull(); err == nil && v != "" {
		return v
	}
	return "1.29"
}

// manifestChannel returns the release channel encoded in the
// Package.appxmanifest (e.g. "beta.1"), searching up from the executable
// directory. It is the single version source for every platform; the
// release version/channel are injected into the manifest by the CI
// pipeline before building.
func manifestChannel() (string, error) {
	dir := filepath.Dir(os.Args[0])
	var lastErr error
	for {
		for _, name := range []string{"Package.appxmanifest", "AppxManifest.xml"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var pkg struct {
				Channel string `xml:"http://schemas.snishaper.dev/release ReleaseChannel"`
			}
			if err := xml.Unmarshal(data, &pkg); err != nil {
				lastErr = err
				continue
			}
			if strings.TrimSpace(pkg.Channel) != "" {
				return strings.TrimSpace(pkg.Channel), nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("manifest not found")
}

// manifestVersionFull returns the full version (with channel suffix when
// the channel is non-stable) from Package.appxmanifest, searching up from
// the executable directory. Mirrors the legacy Windows-only lookup; on
// Linux the same manifest drives the version so both platforms stay in
// sync from a single source.
func manifestVersionFull() (string, error) {
	dir := filepath.Dir(os.Args[0])
	var lastErr error
	identityFallback := ""
	for {
		for _, name := range []string{"Package.appxmanifest", "AppxManifest.xml"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var pkg struct {
				RelVersion string `xml:"http://schemas.snishaper.dev/release Version"`
				RelChannel string `xml:"http://schemas.snishaper.dev/release ReleaseChannel"`
				Identity   struct {
					Version string `xml:"Version,attr"`
				} `xml:"Identity"`
			}
			if err := xml.Unmarshal(data, &pkg); err != nil {
				lastErr = err
				continue
			}
			rv := strings.TrimSpace(pkg.RelVersion)
			rc := strings.TrimSpace(pkg.RelChannel)
			if rv != "" {
				if rc != "" && normalizeReleaseChannel(rc) != "stable" {
					return rv + "-" + rc, nil
				}
				return rv, nil
			}
			if identityFallback == "" && pkg.Identity.Version != "" {
				parts := strings.Split(pkg.Identity.Version, ".")
				for len(parts) > 1 && parts[len(parts)-1] == "0" {
					parts = parts[:len(parts)-1]
				}
				identityFallback = strings.Join(parts, ".")
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if identityFallback != "" {
		return identityFallback, nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("manifest not found")
}
