//go:build windows

package common

import "os/exec"

// OpenTarget hands a file, directory or URL to the shell's default handler.
func OpenTarget(target string) error {
	return exec.Command("cmd", "/c", "start", "", target).Run()
}

// RevealPath opens the file manager with the given file selected.
func RevealPath(path string) error {
	return exec.Command("explorer.exe", "/select,"+path).Run()
}
