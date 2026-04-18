//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	siUser32                   = syscall.NewLazyDLL("user32.dll")
	siKernel32                 = syscall.NewLazyDLL("kernel32.dll")
	siFindWindowW              = siUser32.NewProc("FindWindowW")
	siCreateToolhelp32Snapshot = siKernel32.NewProc("CreateToolhelp32Snapshot")
	siProcess32FirstW          = siKernel32.NewProc("Process32FirstW")
	siProcess32NextW           = siKernel32.NewProc("Process32NextW")
)

const (
	th32csSnapProcess = 0x00000002
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

func recoverBrokenSingleInstance(uniqueID string) {
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
