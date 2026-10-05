//go:build windows && !headless

package app

import (
	"log"
	"runtime"
	"syscall"
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
	procWTSRegisterSessionNotification   = wtsapi32Session.NewProc("WTSRegisterSessionNotification")
	procWTSUnRegisterSessionNotification = wtsapi32Session.NewProc("WTSUnRegisterSessionNotification")
)

const (
	wmWTSessionChange    = 0x02B1
	wtsSessionLock       = 0x7
	wtsSessionUnlock     = 0x8
	notifyForThisSession = 0
	// HWND_MESSAGE = (HWND)-3
	hwndMessage = ^uintptr(2)
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
)

// startSessionWatcher 启动 message-only window 监听 WTS_SESSION_UNLOCK。
// 用户解锁瞬间触发托盘重建，覆盖"锁屏期间 shell 丢弃图标"的场景。
func (a *App) startSessionWatcher() {
	sessionWatcherApp = a
	go runSessionWatcher()
}

func runSessionWatcher() {
	// 消息循环必须绑定到固定 OS 线程
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

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		hwndMessage,
		0, hInstance, 0,
	)
	if hwnd == 0 {
		log.Printf("[tray-session] CreateWindowExW failed: %v", err)
		return
	}
	defer procDestroyWindow.Call(hwnd)

	if ret, _, err := procWTSRegisterSessionNotification.Call(hwnd, notifyForThisSession); ret == 0 {
		log.Printf("[tray-session] WTSRegisterSessionNotification failed: %v", err)
		return
	}
	defer procWTSUnRegisterSessionNotification.Call(hwnd)

	log.Printf("[tray-session] watching for session unlock (hwnd=%x)", hwnd)

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

// sessionWatcherProc 拦截 WM_WTSSESSION_CHANGE 的 WTS_SESSION_UNLOCK。
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
			log.Printf("[tray-session] session unlocked, rebuilding tray")
			if app := sessionWatcherApp; app != nil {
				app.RebuildSystemTray()
			}
		}
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

// isSessionLocked 报告当前会话是否处于锁屏状态。
// 状态未知时返回 false：宁可多重建一次，也不要因状态误判而漏掉重建。
func isSessionLocked() bool {
	return sessionLocked.Load()
}
