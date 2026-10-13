//go:build windows

package app

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	// Legacy HKCU Run value written by older builds. Kept only so an upgrade
	// can remove it: leaving it in place would start a second instance at
	// logon alongside the task.
	autoStartRegistryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	autoStartValueName    = "SniShaper"

	// Task name in the Task Scheduler library.
	autoStartTaskName = "SniShaper"
)

// buildAutoStartCommand builds the command line stored in the task action.
// --startup is added only when the main window should stay hidden on
// auto-start; --autoproxy is added when the proxy should auto-enable on
// auto-start. Manual launches (no args) always show the main window.
func buildAutoStartCommand(execPath string, showMainWindow, autoProxy bool) string {
	trimmed := strings.TrimSpace(execPath)
	if trimmed == "" {
		return ""
	}
	cmd := syscall.EscapeArg(trimmed)
	if !showMainWindow {
		cmd += " --startup"
	}
	if autoProxy {
		cmd += " --autoproxy"
	}
	return cmd
}

// setAutoStartEnabled registers or removes the desktop autostart entry.
//
// The entry is a Task Scheduler task, not an HKCU\...\Run value. A Run value
// is always launched with the plain interactive token, so a program that needs
// administrator rights either silently loses them or triggers UAC on every
// logon. A task with RunLevel=HighestAvailable starts with the user's full
// administrative token and no prompt.
func setAutoStartEnabled(enabled bool, command string) error {
	cleanupLegacyRunValue(autoStartValueName)
	return setTaskAutoStart(autoStartTaskName, enabled, command)
}

// SetNamedAutoStartEntry registers an extra autostart entry under its own task
// name, so the headless service can register independently of the desktop app
// entry.
func SetNamedAutoStartEntry(name string, enabled bool, command string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("autostart entry name is empty")
	}
	cleanupLegacyRunValue(name)
	return setTaskAutoStart(autoStartTaskName+"-"+name, enabled, command)
}

// NamedAutoStartEntryExists reports whether such a task is registered.
func NamedAutoStartEntryExists(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	return taskExists(autoStartTaskName + "-" + name)
}

func setTaskAutoStart(taskName string, enabled bool, command string) error {
	if !enabled {
		return deleteTask(taskName)
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("autostart command is empty")
	}
	exe, args, err := splitCommandLine(command)
	if err != nil {
		return err
	}
	account, err := currentAccountName()
	if err != nil {
		return err
	}
	return createTaskFromXML(taskName, buildAutoStartTaskXML(exe, args, account))
}

// createTaskFromXML overwrites the task named taskName from an XML definition.
//
// Registering a HighestAvailable task requires elevation. When the process is
// not elevated and the task already exists, the existing registration is left
// untouched and nil is returned so a routine sync at startup does not spam
// warnings: the task is already working, and the next elevated launch will
// refresh it. When the task does not exist yet, the caller gets a readable
// error to show the user instead of a silent no-op that never takes effect.
func createTaskFromXML(taskName, xmlContent string) error {
	if !isProcessElevated() {
		if taskExists(taskName) {
			return nil
		}
		return fmt.Errorf("开启开机自启需要管理员权限，请以管理员身份重新运行 SniShaper")
	}

	dir, err := os.MkdirTemp("", "snishaper-task-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	xmlPath := filepath.Join(dir, "task.xml")
	// schtasks only accepts UTF-16LE XML with a BOM. A UTF-8 file is rejected
	// outright, or worse, silently mangled when the account name or the
	// install path contains non-ASCII characters.
	if err := os.WriteFile(xmlPath, utf16LEWithBOM(xmlContent), 0644); err != nil {
		return err
	}

	out, err := runSchtasks("/create", "/tn", taskName, "/xml", xmlPath, "/f")
	if err != nil {
		if out != "" {
			return fmt.Errorf("schtasks /create failed: %s", out)
		}
		return fmt.Errorf("schtasks /create failed: %w", err)
	}
	return nil
}

// deleteTask removes the task if it is registered. /f skips the confirmation
// prompt, which would otherwise block forever under a hidden window.
func deleteTask(taskName string) error {
	if !taskExists(taskName) {
		return nil
	}
	out, err := runSchtasks("/delete", "/tn", taskName, "/f")
	if err != nil {
		if out != "" {
			return fmt.Errorf("schtasks /delete failed: %s", out)
		}
		return fmt.Errorf("schtasks /delete failed: %w", err)
	}
	return nil
}

// taskExists reports whether a task with the given name is registered.
func taskExists(taskName string) bool {
	cmd := exec.Command("schtasks", "/query", "/tn", taskName)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run() == nil
}

func runSchtasks(args ...string) (string, error) {
	cmd := exec.Command("schtasks", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// splitCommandLine splits a command line into its executable path and an
// argument string, following the CreateProcess rules for the leading token:
// a quoted path ends at the closing quote, an unquoted one at the first
// whitespace. The remainder is passed through verbatim, so quoting in the
// argument string is preserved as written.
func splitCommandLine(command string) (string, string, error) {
	rest := strings.TrimLeft(command, " \t")
	if rest == "" {
		return "", "", fmt.Errorf("empty command line")
	}
	var exe string
	if rest[0] == '"' {
		end := strings.IndexByte(rest[1:], '"')
		if end < 0 {
			return "", "", fmt.Errorf("unbalanced quote in command line")
		}
		exe = rest[1 : 1+end]
		rest = rest[2+end:]
	} else if cut := strings.IndexAny(rest, " \t"); cut >= 0 {
		exe = rest[:cut]
		rest = rest[cut:]
	} else {
		exe = rest
		rest = ""
	}
	if strings.TrimSpace(exe) == "" {
		return "", "", fmt.Errorf("empty executable path")
	}
	return exe, strings.TrimSpace(rest), nil
}

// buildAutoStartTaskXML builds the task definition.
//
// Key fields:
//   - LogonTrigger + Principal.UserId: trigger only when this user logs on.
//   - LogonType=InteractiveToken: run with the interactive token. Combined
//     with HighestAvailable this elevates silently for users in the
//     Administrators group. Standard users still see a UAC prompt; that is a
//     Windows token model limit, not something the task can avoid.
//   - RunLevel=HighestAvailable: start with the user's highest token.
//   - ExecutionTimeLimit=PT0S: no time limit. The default 72 hours would kill
//     a long-running proxy.
//   - DisallowStartIfOnBatteries/StopIfGoingOnBatteries=false: start on a
//     laptop that is not plugged in.
func buildAutoStartTaskXML(exe, args, account string) string {
	account = xmlEscape(account)
	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Description>SniShaper auto-start</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>
      <UserId>` + account + `</UserId>
    </LogonTrigger>
  </Triggers>
  <Principals>
    <Principal id="Author">
      <UserId>` + account + `</UserId>
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>false</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>7</Priority>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + xmlEscape(exe) + `</Command>
      <Arguments>` + xmlEscape(args) + `</Arguments>
    </Exec>
  </Actions>
</Task>
`
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func currentAccountName() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("cannot resolve the current user: %w", err)
	}
	name := strings.TrimSpace(u.Username)
	if name == "" {
		return "", fmt.Errorf("current user has no account name")
	}
	return name, nil
}

func isProcessElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

// cleanupLegacyRunValue removes the HKCU Run value written by older builds.
// Without this an upgrade would start two instances at logon: the leftover
// Run entry and the new task.
func cleanupLegacyRunValue(valueName string) {
	key, err := registry.OpenKey(registry.CURRENT_USER, autoStartRegistryPath, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	_ = key.DeleteValue(valueName)
}

func utf16LEWithBOM(s string) []byte {
	units := utf16.Encode([]rune(s))
	buf := make([]byte, 0, 2+len(units)*2)
	buf = append(buf, 0xFF, 0xFE)
	for _, u := range units {
		buf = append(buf, byte(u), byte(u>>8))
	}
	return buf
}