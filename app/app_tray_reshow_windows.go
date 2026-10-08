//go:build windows && !headless

package app

import (
	"log"
	"sync"
	"syscall"
	"unsafe"
)

var (
	user32Reshow     = syscall.NewLazyDLL("user32.dll")
	procFindWindowW  = user32Reshow.NewProc("FindWindowW")
	procPostMessageW = user32Reshow.NewProc("PostMessageW")
	procRegisterWM   = user32Reshow.NewProc("RegisterWindowMessageW")
)

var reshowMsgOnce sync.Once

// reshowMsgID is the system-wide "TaskbarCreated" message id. RegisterWindowMessage
// returns the same id for the same string across the whole system, so this equals
// Wails' internal wmTaskbarCreated
// (v3/pkg/application/application_windows.go:23).
var reshowMsgID uint32

func taskbarCreatedMessageID() uint32 {
	reshowMsgOnce.Do(func() {
		name, _ := syscall.UTF16PtrFromString("TaskbarCreated")
		id, _, _ := procRegisterWM.Call(uintptr(unsafe.Pointer(name)))
		reshowMsgID = uint32(id)
	})
	return reshowMsgID
}

// findWailsMainThreadWindow locates Wails' hidden main-thread window.
//
// Per v3/pkg/application/mainthread_windows.go (initMainLoop):
//
//	m.mainThreadWindowHWND = w32.CreateWindowEx(
//	    0, WndClass, "__wails_hidden_mainthread", WS_DISABLED, ...)
//
// The title is a hardcoded string, hWndParent = 0 (top-level), so
// FindWindow(NULL, "__wails_hidden_mainthread") is sufficient. That window
// exists solely to be a PostMessage target for the runtime, so posting to it
// is a supported, well-defined operation.
//
// FindWindow is called on every invocation: one user32 syscall, negligible
// cost, and it tolerates a potential rebuild of the window.
func findWailsMainThreadWindow() uintptr {
	title, _ := syscall.UTF16PtrFromString("__wails_hidden_mainthread")
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	return hwnd
}

// requestTrayReshow makes Wails run its internal reshowSystrays().
//
// Wails' windowsApp.wndProc handles wmTaskbarCreated as follows
// (application_windows.go):
//
//	case wmTaskbarCreated:
//	    if m.restartingTaskbar.Load() { break }
//	    m.restartingTaskbar.Store(true)
//	    m.reshowSystrays()
//	    go func() { time.Sleep(1000); m.restartingTaskbar.Store(false) }()
//
// reshowSystrays() calls systray.show() for every tray in systrayMap; show()
// issues NIM_ADD + NIM_SETVERSION against the existing HWND/UID, so it is
// idempotent: if the shell still holds the icon it is a no-op refresh; if the
// shell dropped it, the original slot is restored. It never registers a
// second, independent icon.
//
// See v3/pkg/application/application_windows.go (reshowSystrays) and
// systemtray_windows.go (windowsSystemTray.show).
func requestTrayReshow() {
	msgID := taskbarCreatedMessageID()
	if msgID == 0 {
		log.Printf("[tray] RegisterWindowMessage(TaskbarCreated) failed")
		return
	}

	hwnd := findWailsMainThreadWindow()
	if hwnd == 0 {
		log.Printf("[tray] Wails main thread window not found")
		return
	}

	procPostMessageW.Call(hwnd, uintptr(msgID), 0, 0)
}