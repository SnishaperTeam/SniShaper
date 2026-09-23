//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// macOS autostart is a per user launch agent: the plist lives in
// ~/Library/LaunchAgents and launchd starts it with the user's session.

const launchAgentLabel = "com.snishaper.desktop"

func buildAutoStartCommand(execPath string, showMainWindow, autoProxy bool) string {
	args := []string{execPath}
	if !showMainWindow {
		args = append(args, "--startup")
	}
	if autoProxy {
		args = append(args, "--autoproxy")
	}
	return strings.Join(args, " ")
}

func launchAgentPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func setAutoStartEnabled(enabled bool, command string) error {
	path := launchAgentPath()
	if path == "" {
		return fmt.Errorf("cannot resolve the home directory")
	}

	if !enabled {
		// Unloading an agent that is not registered is the normal case when
		// autostart is off, so both steps tolerate a missing entry.
		_ = exec.Command("launchctl", "bootout", "gui/"+strconv.Itoa(os.Getuid()), path).Run()
		_ = exec.Command("launchctl", "unload", path).Run()
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("autostart command is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(launchAgentPlist(command)), 0644); err != nil {
		return err
	}

	domain := "gui/" + strconv.Itoa(os.Getuid())
	if err := exec.Command("launchctl", "bootstrap", domain, path).Run(); err != nil {
		// bootstrap exists since 10.11; fall back for older systems.
		if err := exec.Command("launchctl", "load", "-w", path).Run(); err != nil {
			return fmt.Errorf("launchctl could not load %s: %w", path, err)
		}
	}
	return nil
}

func launchAgentPlist(command string) string {
	// ProgramArguments runs the command through /bin/sh so the startup flags
	// built by buildAutoStartCommand are applied verbatim.
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchAgentLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/sh</string>
		<string>-c</string>
		<string>` + escapePlistText(command) + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`
}

func escapePlistText(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}
