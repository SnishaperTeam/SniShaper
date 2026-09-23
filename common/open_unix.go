//go:build !windows && !darwin

package common

import (
	"os/exec"
	"path/filepath"
)

// OpenTarget hands a file, directory or URL to the desktop's default handler.
// xdg-open is the freedesktop entry point and is present on every mainstream
// distribution that ships a desktop session.
func OpenTarget(target string) error {
	return exec.Command("xdg-open", target).Run()
}

// RevealPath has no portable equivalent on Linux; opening the containing
// directory is the closest behaviour that still shows the file to the user.
func RevealPath(path string) error {
	return exec.Command("xdg-open", filepath.Dir(path)).Run()
}
