//go:build windows && !headless

package app

import (
	"log"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32Session   = syscall.NewLazyDLL("user32.dll")
	kernel32Session = syscall.NewLazyDLL("kernel32.dll")
	wtsapi32Session = syscall.NewLazyDLL("wtsapi32.dll")

	procRegisterClassExW                 = user32Session.NewProc("RegisterClassExW")
	procCreateWindowExW                  = user32Session.NewProc("CreateWindowExW")
	procDefWindowProcW                   = user32Session.NewProc("DefWindowProcW")
	procGetMessageW                      = user32Session.NewProc("GetMessageW")
	procTranslateMessage                 = user32Session.NewProc("TranslateMessage")
	procDispatchMessageW                 = user32Session.NewProc("DispatchMessageW")
	procDestroyWindow                    = user32Session.NewProc("DestroyWindow")
	procUnregisterClassW                 = user32Session.NewProc("UnregisterClassW")
	procGetModuleHandleW                 = kernel32Session.NewProc("GetModuleHandleW")
	procRegisterWindowMessageW           = user32Session.NewProc("RegisterWindowMessageW")
	procChangeWindowMessageFilterEx      = user32Session.NewProc("ChangeWindowMessageFilterEx")
	procWTSRegisterSessionNotification   = wtsapi32Session.NewProc("WTSRegisterSessionNotification")
	procWTSUnRegisterSessionNotification = wtsapi32Session.NewProc("WTSUnRegisterSessionNotification")
)

const (
	wmWTSessionChange    = 0x02B1
	wtsSessionLock       = 0x7
	wtsSessionUnlock     = 0x8
	notifyForThisSession = 0
)

const sessionWatcherClassName = "SniShaperSessionWatcher_v1"

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type winMsg struct {
	Hwnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

var (
	sessionWatcherWndProc = syscall.NewCallback(sessionWatcherProc)
	sessionWatcherApp     *App
	sessionLocked         atomic.Bool
	taskbarCreatedMsg     uint32
)

// startSessionWatcher starts a hidden window that listens for the shell
// events that make the tray icon disappear:
//
//   - WM_WTSSESSION_CHANGE / WTS_SESSION_UNLOCK — the icon registered while
//     the session was locked is discarded when the user logs in.
//   - TaskbarCreated — explorer.exe crash or restart rebuilds the shell and
//     drops every tray icon along with it. Wails' own wndProc also handles
//     this broadcast, but only when the app runs at medium integrity:
//     explorer broadcasts at medium IL, and UIPI blocks cross-integrity
//     window messages into an elevated process. Wails never calls
//     ChangeWindowMessageFilterEx, so in an elevated build its handler is
//     dead code and this watcher is the only path that sees the broadcast.
//
// The window is a normal top-level window created without WS_VISIBLE, so it
// never shows on screen but does receive HWND_BROADCAST messages. A
// message-only window would not: broadcasts are not delivered to message-only
// windows, which is why the class is not created with HWND_MESSAGE.
func (a *App) startSessionWatcher() {
	sessionWatcherApp = a
	go runSessionWatcher()
}

func runSessionWatcher() {
	// The message loop must stay on a single OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance, _, _ := procGetModuleHandleW.Call(0)

	className, _ := syscall.UTF16PtrFromString(sessionWatcherClassName)
	windowName, _ := syscall.UTF16PtrFromString(sessionWatcherClassName)

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		LpfnWndProc:   sessionWatcherWndProc,
		HInstance:     syscall.Handle(hInstance),
		LpszClassName: className,
	}
	if ret, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); ret == 0 {
		log.Printf("[tray-session] RegisterClassExW failed: %v", err)
		return
	}
	defer procUnregisterClassW.Call(uintptr(unsafe.Pointer(className)), hInstance)

	// Register the "TaskbarCreated" broadcast message id. explorer.exe sends
	// it to every top-level window whenever it (re)starts.
	if taskbarCreatedMsg == 0 {
		name, _ := syscall.UTF16PtrFromString("TaskbarCreated")
		id, _, _ := procRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(name)))
		taskbarCreatedMsg = uint32(id)
	}

	// Parent = 0 (desktop) so the window is a normal top-level window and
	// receives broadcasts. No WS_VISIBLE flag, so it stays hidden.
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		0,
		0, hInstance, 0,
	)
	if hwnd == 0 {
		log.Printf("[tray-session] CreateWindowExW failed: %v", err)
		return
	}
	defer procDestroyWindow.Call(hwnd)

	// TaskbarCreated is broadcast by explorer.exe, which runs at a lower
	// integrity level than this elevated app. UIPI blocks the message by
	// default, so explicitly allow it through. WM_WTSSESSION_CHANGE comes
	// from the system session manager and is not subject to UIPI.
	if taskbarCreatedMsg != 0 {
		procChangeWindowMessageFilterEx.Call(hwnd, uintptr(taskbarCreatedMsg), msgfltAllow, 0)
	}

	if ret, _, err := procWTSRegisterSessionNotification.Call(hwnd, notifyForThisSession); ret == 0 {
		log.Printf("[tray-session] WTSRegisterSessionNotification failed: %v", err)
		return
	}
	defer procWTSUnRegisterSessionNotification.Call(hwnd)

	log.Printf("[tray-session] watching for session unlock and taskbar creation (hwnd=%x)", hwnd)

	var msg winMsg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// sessionWatcherProc handles the two shell events that drop tray icons.
func sessionWatcherProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[tray-session] panic in wndproc: %v", r)
		}
	}()

	if msg == wmWTSessionChange {
		switch wParam {
		case wtsSessionLock:
			sessionLocked.Store(true)
			log.Printf("[tray-session] session locked")
		case wtsSessionUnlock:
			sessionLocked.Store(false)
			log.Printf("[tray-session] session unlocked, requesting tray reshow")
			if app := sessionWatcherApp; app != nil {
				app.RebuildSystemTray()
			}
		}
	}

	// explorer.exe broadcasts TaskbarCreated every time the shell starts or
	// restarts, but the notification area is not ready to accept new
	// registrations the instant the broadcast fires. There is no "ready"
	// signal to wait for, so retry a few times. The intervals are wider than
	// both debounce windows (RebuildSystemTray's own 3 s window and Wails'
	// internal 1 s TaskbarCreated debounce), so every retry actually runs.
	//
	// RebuildSystemTray no longer creates a new tray — it requests Wails to
	// re-add the existing one via NIM_ADD on the same HWND/UID, so the retries
	// are idempotent and never produce a duplicate icon.
	if taskbarCreatedMsg != 0 && msg == uintptr(taskbarCreatedMsg) {
		log.Printf("[tray-session] taskbar created (explorer restart), scheduling reshow retries")
		if app := sessionWatcherApp; app != nil {
			go func() {
				for _, d := range []time.Duration{
					1 * time.Second,
					4 * time.Second,
					16 * time.Second,
				} {
					time.Sleep(d)
					app.RebuildSystemTray()
				}
			}()
		}
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

// isSessionLocked reports whether the current session is locked. The zero
// value is false: when the state is unknown, prefer rebuilding once too many
// rather than skipping a rebuild that was actually needed.
func isSessionLocked() bool {
	return sessionLocked.Load()
}