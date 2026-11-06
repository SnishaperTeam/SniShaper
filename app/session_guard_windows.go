//go:build windows

package app

import (
	"os"

	"golang.org/x/sys/windows"
)

var (
	user32SessionGuard   = windows.NewLazySystemDLL("user32.dll")
	procOpenInputDesktop = user32SessionGuard.NewProc("OpenInputDesktop")
	procCloseDesktop     = user32SessionGuard.NewProc("CloseDesktop")

	kernel32Console   = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = kernel32Console.NewProc("AttachConsole")
)

// NonInteractiveSession reports whether the process runs without an
// interactive desktop: session 0 (services, scheduled automation) or a
// window station with no input desktop. A GUI cannot run there; the desktop
// binary falls back to its bundled CLI mode instead of letting the window
// toolkit panic in a headless context. winget's post-install executable
// validation launches the installed binary in exactly such environments.
func NonInteractiveSession() bool {
	var session uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err == nil && session == 0 {
		return true
	}
	if err := procOpenInputDesktop.Find(); err != nil {
		// API unavailable: cannot prove absence of a desktop, stay GUI.
		return false
	}
	h, _, _ := procOpenInputDesktop.Call(0, 0, 0x0001) // DESKTOP_READOBJECTS
	if h == 0 {
		return true
	}
	procCloseDesktop.Call(h)
	return false
}

// AttachParentConsole reattaches stdout/stderr to the console of the parent
// process. The desktop build is a windowsgui binary, so output from the
// bundled CLI mode would otherwise vanish when invoked from a terminal.
// A stdout that already points somewhere (pipe, redirected file) is left
// untouched so console attachment never hijacks an existing redirect.
func AttachParentConsole() {
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil &&
		h != windows.InvalidHandle && h != 0 {
		return
	}
	if err := procAttachConsole.Find(); err != nil {
		return
	}
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	if r, _, _ := procAttachConsole.Call(attachParentProcess); r == 0 {
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout = out
		os.Stderr = out
	}
}
