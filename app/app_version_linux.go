//go:build linux

package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func manifestChannel() (string, error) {
	dir := filepath.Dir(os.Args[0])
	var lastErr error
	for {
		data, err := os.ReadFile(filepath.Join(dir, "version.json"))
		if err != nil {
			lastErr = err
		} else {
			var v struct {
				ReleaseChannel string `json:"releaseChannel"`
			}
			if err := json.Unmarshal(data, &v); err != nil {
				lastErr = err
			} else if strings.TrimSpace(v.ReleaseChannel) != "" {
				return strings.TrimSpace(v.ReleaseChannel), nil
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
	return "", fmt.Errorf("version.json not found")
}

// manifestVersionFull returns the full version (with channel suffix when the
// channel is non-stable) from version.json, searching up from the executable
// directory. Mirrors the Windows appxmanifest lookup, but backed by JSON.

func manifestVersionFull() (string, error) {
	dir := filepath.Dir(os.Args[0])
	var lastErr error
	for {
		data, err := os.ReadFile(filepath.Join(dir, "version.json"))
		if err != nil {
			lastErr = err
		} else {
			var v struct {
				Version        string `json:"version"`
				ReleaseChannel string `json:"releaseChannel"`
			}
			if err := json.Unmarshal(data, &v); err != nil {
				lastErr = err
			} else {
				rv := strings.TrimSpace(v.Version)
				rc := strings.TrimSpace(v.ReleaseChannel)
				if rv != "" {
					if rc != "" && normalizeReleaseChannel(rc) != "stable" {
						return rv + "-" + rc, nil
					}
					return rv, nil
				}
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
	return "", fmt.Errorf("version.json not found")
}
