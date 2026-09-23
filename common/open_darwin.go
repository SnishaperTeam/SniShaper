//go:build darwin

package common

import "os/exec"

// OpenTarget hands a file, directory or URL to the default application.
func OpenTarget(target string) error {
	return exec.Command("open", target).Run()
}

// RevealPath opens Finder with the given file selected.
func RevealPath(path string) error {
	return exec.Command("open", "-R", path).Run()
}
