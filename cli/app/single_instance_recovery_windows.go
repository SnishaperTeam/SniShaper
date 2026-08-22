//go:build windows

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const wmCopyDataSingleInstanceData = 1542

var (
	siUser32                      = syscall.NewLazyDLL("user32.dll")
	siKernel32                    = syscall.NewLazyDLL("kernel32.dll")
	siFindWindowW                 = siUser32.NewProc("FindWindowW")
	siCreateToolhelp32Snapshot    = siKernel32.NewProc("CreateToolhelp32Snapshot")
	siProcess32FirstW             = siKernel32.NewProc("Process32FirstW")
	siProcess32NextW              = siKernel32.NewProc("Process32NextW")
	siChangeWindowMessageFilterEx = siUser32.NewProc("ChangeWindowMessageFilterEx")
	siSendMessageTimeoutW         = siUser32.NewProc("SendMessageTimeoutW")
	siGetWindowThreadProcessId    = siUser32.NewProc("GetWindowThreadProcessId")
)

const (
	th32csSnapProcess = 0x00000002
	wmCopyData        = 0x004A
	msgfltAllow       = 1
	smtoBlock         = 0x0001
	smtoAbortIfHung   = 0x0002
)

type processEntry32 struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PcPriClassBase  int32
	Flags           uint32
	ExeFile         [windows.MAX_PATH]uint16
}

// RecoverBrokenSingleInstance recovers from a broken single instance state.
func RecoverBrokenSingleInstance(uniqueID string) {
	if strings.TrimSpace(uniqueID) == "" {
		return
	}

	id := "wails-app-" + uniqueID
	className := id + "-sic"
	windowName := id + "-siw"
	mutexName := id + "-sim"

	if findSingleInstanceWindow(className, windowName) != 0 {
		return
	}

	mutex, err := windows.OpenMutex(windows.SYNCHRONIZE, false, windows.StringToUTF16Ptr(mutexName))
	if err != nil {
		return
	}
	_ = windows.CloseHandle(mutex)

	time.Sleep(250 * time.Millisecond)
	if findSingleInstanceWindow(className, windowName) != 0 {
		return
	}

	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeName := strings.ToLower(filepath.Base(exePath))
	if exeName == "" {
		return
	}

	currentPID := uint32(os.Getpid())
	killedAny := false
	for _, pid := range findProcessesByName(exeName) {
		if pid == 0 || pid == currentPID {
			continue
		}
		proc, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
		if err != nil {
			continue
		}
		_ = windows.TerminateProcess(proc, 0)
		_ = windows.CloseHandle(proc)
		killedAny = true
	}
	if killedAny {
		// Wait for the mutex to be released so a fresh app can acquire it.
		waitForMutexReleased(mutexName)
	}
}

func waitForMutexReleased(mutexName string) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mutex, err := windows.OpenMutex(windows.SYNCHRONIZE, false, windows.StringToUTF16Ptr(mutexName))
		if err != nil {
			return
		}
		_ = windows.CloseHandle(mutex)
		time.Sleep(50 * time.Millisecond)
	}
}

func findSingleInstanceWindow(className, windowName string) uintptr {
	classPtr, err := windows.UTF16PtrFromString(className)
	if err != nil {
		return 0
	}
	windowPtr, err := windows.UTF16PtrFromString(windowName)
	if err != nil {
		return 0
	}
	hwnd, _, _ := siFindWindowW.Call(
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(windowPtr)),
	)
	return hwnd
}

func AllowSingleInstanceCrossIntegrity(uniqueID string) {
	if err := siChangeWindowMessageFilterEx.Find(); err != nil {
		return
	}
	id := "wails-app-" + uniqueID
	className := id + "-sic"
	windowName := id + "-siw"
	hwnd := findSingleInstanceWindow(className, windowName)
	if hwnd == 0 {
		return
	}
	siChangeWindowMessageFilterEx.Call(hwnd, uintptr(wmCopyData), uintptr(msgfltAllow), 0)
}

// IsSingleInstanceRunning reports whether another instance owns the single-instance window.
func IsSingleInstanceRunning(uniqueID string) bool {
	if strings.TrimSpace(uniqueID) == "" {
		return false
	}
	id := "wails-app-" + uniqueID
	return findSingleInstanceWindow(id+"-sic", id+"-siw") != 0
}

// WakeSingleInstance pings the running instance with a WM_COPYDATA message in the
// same format Wails uses, triggering OnSecondInstanceLaunch on the first instance.
// Returns nil when the message was delivered.
// If the first instance runs at a higher integrity level (elevated), UIPI blocks
// WM_COPYDATA from a lower-integrity sender. This function calls
// ChangeWindowMessageFilterEx on the target window to allow WM_COPYDATA across
// integrity boundaries, then retries the send.
func WakeSingleInstance(uniqueID string) error {
	if strings.TrimSpace(uniqueID) == "" {
		return nil
	}
	id := "wails-app-" + uniqueID
	hwnd := findSingleInstanceWindow(id+"-sic", id+"-siw")
	if hwnd == 0 {
		return syscall.Errno(windows.ERROR_FILE_NOT_FOUND)
	}

	data, err := json.Marshal(struct {
		Args       []string `json:"args"`
		WorkingDir string   `json:"workingDir"`
	}{Args: os.Args, WorkingDir: mustGetwd()})
	if err != nil {
		return err
	}

	dataUTF16, err := windows.UTF16FromString(string(data))
	if err != nil {
		return err
	}

	type copyDataStruct struct {
		DwData uintptr
		CbData uint32
		LpData uintptr
	}
	cds := copyDataStruct{
		DwData: wmCopyDataSingleInstanceData,
		CbData: uint32((len(dataUTF16) * 2) + 1),
		LpData: uintptr(unsafe.Pointer(&dataUTF16[0])),
	}

	var result uintptr
	ret, _, _ := siSendMessageTimeoutW.Call(
		hwnd,
		uintptr(wmCopyData),
		0,
		uintptr(unsafe.Pointer(&cds)),
		uintptr(smtoBlock|smtoAbortIfHung),
		3000,
		uintptr(unsafe.Pointer(&result)),
	)
	if ret == 0 {
		// UIPI may be blocking WM_COPYDATA from a lower-integrity sender.
		// Allow WM_COPYDATA on the target window and retry.
		if siChangeWindowMessageFilterEx.Find() == nil {
			siChangeWindowMessageFilterEx.Call(
				hwnd,
				uintptr(wmCopyData),
				uintptr(msgfltAllow),
				0,
			)
		}
		ret, _, _ = siSendMessageTimeoutW.Call(
			hwnd,
			uintptr(wmCopyData),
			0,
			uintptr(unsafe.Pointer(&cds)),
			uintptr(smtoBlock|smtoAbortIfHung),
			3000,
			uintptr(unsafe.Pointer(&result)),
		)
		if ret == 0 {
			return syscall.Errno(windows.ERROR_ACCESS_DENIED)
		}
	}
	return nil
}

// KillSingleInstance force-terminates the process that owns the single-instance
// window and waits until its mutex/window are released, so a subsequent
// CreateMutex in application.New() does not mis-detect "already running".
func KillSingleInstance(uniqueID string) {
	if strings.TrimSpace(uniqueID) == "" {
		return
	}
	id := "wails-app-" + uniqueID
	hwnd := findSingleInstanceWindow(id+"-sic", id+"-siw")
	if hwnd == 0 {
		return
	}
	var pid uint32
	siGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 || pid == uint32(os.Getpid()) {
		return
	}
	proc, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return
	}
	_ = windows.TerminateProcess(proc, 0)
	_ = windows.CloseHandle(proc)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !IsSingleInstanceRunning(uniqueID) && !singleInstanceMutexExists(uniqueID) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func singleInstanceMutexExists(uniqueID string) bool {
	if strings.TrimSpace(uniqueID) == "" {
		return false
	}
	id := "wails-app-" + uniqueID
	mutex, err := windows.OpenMutex(windows.SYNCHRONIZE, false, windows.StringToUTF16Ptr(id+"-sim"))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(mutex)
	return true
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

func findProcessesByName(name string) []uint32 {
	snapshot, _, _ := siCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == uintptr(windows.InvalidHandle) {
		return nil
	}
	defer windows.CloseHandle(windows.Handle(snapshot))

	entry := processEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))

	var pids []uint32
	ret, _, _ := siProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		exe := strings.ToLower(windows.UTF16ToString(entry.ExeFile[:]))
		if exe == name {
			pids = append(pids, entry.ProcessID)
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, _ = siProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return pids
}
