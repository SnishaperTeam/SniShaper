//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	th32csSnapProcess            = 0x00000002
	processQueryLimitedInformation = 0x1000
	processTerminate              = 0x0001
	wmClose                       = 0x0010
	mbOK                          = 0x00000000
	mbOKCancel                    = 0x00000001
	mbIconWarning                 = 0x00000030
	mbIconError                   = 0x00000010
	mbIconInformation             = 0x00000040
	idOK                          = 1
	swShowNormal                  = 1
	seeMaskNoCloseProcess         = 0x00000040
	seeMaskNoAsync                = 0x00000100
	waitInfinite                  = 0xFFFFFFFF
	errorCancelled                = 1223
	moveFileReplaceExisting       = 0x00000001
	moveFileDelayUntilReboot      = 0x00000004
	invalidHandleValue            = ^uintptr(0)
)

var (
	modKernel32 = syscall.NewLazyDLL("kernel32.dll")
	modUser32   = syscall.NewLazyDLL("user32.dll")
	modShell32  = syscall.NewLazyDLL("shell32.dll")
	modWintrust = syscall.NewLazyDLL("wintrust.dll")

	procCreateToolhelp32Snapshot     = modKernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW              = modKernel32.NewProc("Process32FirstW")
	procProcess32NextW               = modKernel32.NewProc("Process32NextW")
	procOpenProcess                  = modKernel32.NewProc("OpenProcess")
	procCloseHandle                  = modKernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW   = modKernel32.NewProc("QueryFullProcessImageNameW")
	procTerminateProcess             = modKernel32.NewProc("TerminateProcess")
	procWaitForSingleObject          = modKernel32.NewProc("WaitForSingleObject")
	procGetExitCodeProcess           = modKernel32.NewProc("GetExitCodeProcess")
	procMoveFileExW                  = modKernel32.NewProc("MoveFileExW")

	procMessageBoxW                = modUser32.NewProc("MessageBoxW")
	procEnumWindows                = modUser32.NewProc("EnumWindows")
	procGetWindowThreadProcessId   = modUser32.NewProc("GetWindowThreadProcessId")
	procPostMessageW               = modUser32.NewProc("PostMessageW")

	procShellExecuteExW = modShell32.NewProc("ShellExecuteExW")
	procWinVerifyTrust  = modWintrust.NewProc("WinVerifyTrust")
)

type processEntry32W struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}

type shellExecuteInfoW struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         uintptr
	LpVerb       uintptr
	LpFile       uintptr
	LpParameters uintptr
	LpDirectory  uintptr
	NShow        int32
	HInstApp     uintptr
	LpIDList     uintptr
	LpClass      uintptr
	HKeyClass    uintptr
	DwHotKey     uint32
	HIcon        uintptr
	HProcess     uintptr
}

type targetProcess struct {
	PID  uint32
	Name string
	Path string
}

func messageBox(text, title string, flags uint32) int {
	t, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return 2
	}
	c, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 2
	}
	r, _, _ := procMessageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), uintptr(flags))
	runtime.KeepAlive(t)
	runtime.KeepAlive(c)
	return int(r)
}

func showError(err error, logPath string) {
	messageBox("更新失败："+err.Error()+"\n\n旧版本未被修改，仍可正常运行。\n日志："+logPath, "SniShaper 更新器", mbOK|mbIconError)
}

func showInfo(text string) {
	messageBox(text, "SniShaper 更新器", mbOK|mbIconInformation)
}

func pathWithin(path, dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return true
	}
	p := strings.ToLower(filepath.Clean(path))
	d := strings.ToLower(filepath.Clean(dir))
	if !strings.HasSuffix(d, string(os.PathSeparator)) {
		d += string(os.PathSeparator)
	}
	return strings.HasPrefix(p, d)
}

func matchProcessName(name string, names []string) bool {
	for _, n := range names {
		if strings.EqualFold(name, n) {
			return true
		}
	}
	return false
}

func queryFullImageName(pid uint32) string {
	h, _, _ := procOpenProcess.Call(uintptr(processQueryLimitedInformation), 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	runtime.KeepAlive(buf)
	if r == 0 || size == 0 || int(size) > len(buf) {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}

func snapshotProcesses(names []string, installDir string, selfPID uint32) ([]targetProcess, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(uintptr(th32csSnapProcess), 0)
	if snap == invalidHandleValue {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)
	var entry processEntry32W
	entry.DwSize = uint32(unsafe.Sizeof(entry))
	r, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	list := []targetProcess{}
	for r != 0 {
		name := syscall.UTF16ToString(entry.SzExeFile[:])
		if matchProcessName(name, names) && entry.Th32ProcessID != selfPID {
			full := queryFullImageName(entry.Th32ProcessID)
			if full == "" || pathWithin(full, installDir) {
				list = append(list, targetProcess{PID: entry.Th32ProcessID, Name: name, Path: full})
			}
		}
		r, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return list, nil
}

func closeWindowsOf(pids map[uint32]bool) {
	cb := syscall.NewCallback(func(hwnd, lparam uintptr) uintptr {
		var pid uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pids[pid] {
			procPostMessageW.Call(hwnd, uintptr(wmClose), 0, 0)
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
}

func terminateProcess(pid uint32) error {
	h, _, _ := procOpenProcess.Call(uintptr(processTerminate), 0, uintptr(pid))
	if h == 0 {
		return fmt.Errorf("OpenProcess failed for pid %d", pid)
	}
	defer procCloseHandle.Call(h)
	r, _, callErr := procTerminateProcess.Call(h, 1)
	if r == 0 {
		return fmt.Errorf("TerminateProcess failed for pid %d: %v", pid, callErr)
	}
	return nil
}

func closeProcesses(c Config, installDir string, l *Logger) error {
	selfPID := uint32(os.Getpid())
	targets, err := snapshotProcesses(c.ProcessNames, installDir, selfPID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		l.Logf("no running application processes to close")
		return nil
	}
	names := []string{}
	for _, t := range targets {
		names = append(names, fmt.Sprintf("%s (pid %d)", t.Name, t.PID))
	}
	prompt := "检测到以下程序正在运行：\n" + strings.Join(names, "\n") + "\n\n安装前需要关闭它们。请先保存好数据，然后点「确定」继续；点「取消」放弃本次更新。"
	if messageBox(prompt, "SniShaper 更新器", mbOKCancel|mbIconWarning) != idOK {
		return errUserCancelled
	}
	pids := map[uint32]bool{}
	for _, t := range targets {
		pids[t.PID] = true
	}
	closeWindowsOf(pids)
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		remaining, err := snapshotProcesses(c.ProcessNames, installDir, selfPID)
		if err == nil && len(remaining) == 0 {
			l.Logf("application processes exited after WM_CLOSE")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	remaining, err := snapshotProcesses(c.ProcessNames, installDir, selfPID)
	if err != nil {
		return err
	}
	if len(remaining) == 0 {
		return nil
	}
	forcePrompt := "以下程序仍未退出，文件可能被占用：\n" + strings.Join(names, "\n") + "\n\n点「确定」强制结束它们（未保存数据会丢失），点「取消」放弃本次更新。"
	if messageBox(forcePrompt, "SniShaper 更新器", mbOKCancel|mbIconWarning) != idOK {
		return errUserCancelled
	}
	for _, t := range remaining {
		if err := terminateProcess(t.PID); err != nil {
			l.Logf("terminate failed: %v", err)
		}
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		remaining, err = snapshotProcesses(c.ProcessNames, installDir, selfPID)
		if err == nil && len(remaining) == 0 {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("application processes still running, installation aborted")
}

func shellExecute(verb, file, params, dir string, wait bool) (uint32, error) {
	v, err := syscall.UTF16PtrFromString(verb)
	if err != nil {
		return 0, err
	}
	f, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return 0, err
	}
	p, err := syscall.UTF16PtrFromString(params)
	if err != nil {
		return 0, err
	}
	d, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var sei shellExecuteInfoW
	sei.CbSize = uint32(unsafe.Sizeof(sei))
	sei.FMask = seeMaskNoAsync
	if wait {
		sei.FMask |= seeMaskNoCloseProcess
	}
	sei.LpVerb = uintptr(unsafe.Pointer(v))
	sei.LpFile = uintptr(unsafe.Pointer(f))
	sei.LpParameters = uintptr(unsafe.Pointer(p))
	sei.LpDirectory = uintptr(unsafe.Pointer(d))
	sei.NShow = swShowNormal
	r, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	runtime.KeepAlive(v)
	runtime.KeepAlive(f)
	runtime.KeepAlive(p)
	runtime.KeepAlive(d)
	if r == 0 {
		if callErr == syscall.Errno(errorCancelled) {
			return 0, errElevationCancelled
		}
		return 0, fmt.Errorf("ShellExecuteEx(%s, %s) failed: %v", verb, file, callErr)
	}
	if !wait {
		return 0, nil
	}
	if sei.HProcess == 0 {
		return 0, errors.New("ShellExecuteEx returned no process handle")
	}
	defer procCloseHandle.Call(sei.HProcess)
	waitResult, _, _ := procWaitForSingleObject.Call(sei.HProcess, uintptr(waitInfinite))
	if waitResult != 0 {
		return 0, fmt.Errorf("WaitForSingleObject returned 0x%X", uint32(waitResult))
	}
	var code uint32
	ok, _, callErr := procGetExitCodeProcess.Call(sei.HProcess, uintptr(unsafe.Pointer(&code)))
	if ok == 0 {
		return 0, fmt.Errorf("GetExitCodeProcess failed: %v", callErr)
	}
	return code, nil
}

func scheduleReplacement(src, dst string) error {
	s, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	d, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	r, _, callErr := procMoveFileExW.Call(
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(d)),
		uintptr(moveFileReplaceExisting|moveFileDelayUntilReboot),
	)
	runtime.KeepAlive(s)
	runtime.KeepAlive(d)
	if r == 0 {
		return fmt.Errorf("MoveFileExW(DELAY_UNTIL_REBOOT) failed: %v", callErr)
	}
	return nil
}

func scheduleDeletion(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	r, _, callErr := procMoveFileExW.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(moveFileDelayUntilReboot))
	runtime.KeepAlive(p)
	if r == 0 {
		return fmt.Errorf("MoveFileExW(DELETE) failed: %v", callErr)
	}
	return nil
}

func innoExitText(code uint32) string {
	switch code {
	case 0:
		return "setup completed successfully"
	case 1:
		return "setup failed to initialize"
	case 2:
		return "user cancelled before installation started"
	case 3:
		return "fatal error preparing installation"
	case 4:
		return "fatal error during installation"
	case 5:
		return "user cancelled during installation"
	case 6:
		return "setup terminated by debugger"
	case 7:
		return "preparing to install determined setup cannot proceed"
	case 8:
		return "preparing to install determined a restart is required"
	default:
		return "unknown installer exit code"
	}
}
