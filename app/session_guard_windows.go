//go:build windows

package app

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// Replaces the unreliable user32.dll OpenInputDesktop API with WTSAPI32.
	wtsapi32                        = windows.NewLazySystemDLL("wtsapi32.dll")
	procWTSQuerySessionInformationW = wtsapi32.NewProc("WTSQuerySessionInformationW")
	procWTSFreeMemory               = wtsapi32.NewProc("WTSFreeMemory")

	// kernel32Console remains in its original position.
	kernel32Console   = windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole = kernel32Console.NewProc("AttachConsole")
)

const (
	wtsCurrentServerHandle = 0
	wtsConnectState        = 8 // WTS_INFO_CLASS.WTSConnectState
	wtsActive              = 0 // WTS_CONNECTSTATE_CLASS.WTSActive
)

// NonInteractiveSession reports whether the process runs without an
// interactive desktop. This happens in session 0 (services, scheduled
// automation) or when the user session is not yet fully active (e.g., during
// the early logon phase).
//
// Using WTSQuerySessionInformation is far more reliable than OpenInputDesktop,
// which frequently returns false negatives right after logon before the shell
// finishes initializing, causing the GUI to incorrectly fall back to CLI mode.
func NonInteractiveSession() bool {
	var session uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &session); err == nil && session == 0 {
		return true
	}

	// Query the WTS connection state. If the session is not yet WTSActive
	// (e.g., WTSConnected or WTSInitializing), the interactive desktop is
	// not ready.
	if err := procWTSQuerySessionInformationW.Find(); err == nil {
		var pBuffer unsafe.Pointer
		var bytesReturned uint32
		r, _, _ := procWTSQuerySessionInformationW.Call(
			uintptr(wtsCurrentServerHandle),
			uintptr(session),
			uintptr(wtsConnectState),
			uintptr(unsafe.Pointer(&pBuffer)),
			uintptr(unsafe.Pointer(&bytesReturned)),
		)
		if r != 0 && pBuffer != nil {
			defer procWTSFreeMemory.Call(uintptr(pBuffer))
			state := *(*uint32)(pBuffer)
			return state != wtsActive
		}
	}

	// If the WTS API fails (extremely unlikely on modern Windows), we cannot
	// definitively prove the desktop is absent. Default to GUI mode to avoid
	// killing the app silently.
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
