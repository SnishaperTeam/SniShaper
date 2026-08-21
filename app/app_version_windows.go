//go:build windows

package app

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
