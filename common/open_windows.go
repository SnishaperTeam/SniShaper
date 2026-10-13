//go:build windows

package common

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

const swShownormal = 1

var (
	openShell32                    = syscall.NewLazyDLL("shell32.dll")
	procILCreateFromPathW          = openShell32.NewProc("ILCreateFromPathW")
	procILFree                     = openShell32.NewProc("ILFree")
	procSHOpenFolderAndSelectItems = openShell32.NewProc("SHOpenFolderAndSelectItems")
	procShellExecuteW              = openShell32.NewProc("ShellExecuteW")
)

// OpenTarget hands a file, directory or URL to the shell's default handler.
//
// Uses ShellExecuteW, the documented shell entry point for "open this with the
// default handler". The previous `cmd /c start "" <target>` form sent the
// target through a second parser inside cmd.exe, where exec.Command does not
// escape shell metacharacters: a path containing &, |, ^ or % behaved
// unpredictably.
func OpenTarget(target string) error {
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}

	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		0,
		0,
		swShownormal,
	)
	// ShellExecuteW returns a value greater than 32 on success. Anything at or
	// below that is an error code, not a handle, so it must be reported: a
	// silent failure here means the user clicked "open folder" and nothing
	// happened.
	if ret <= 32 {
		return fmt.Errorf("ShellExecuteW failed for %q: code %d", target, ret)
	}
	return nil
}

// RevealPath opens the file manager with the given file selected.
//
// Uses SHOpenFolderAndSelectItems, the documented shell API for exactly this
// task, instead of the undocumented "explorer.exe /select,<path>" convention.
// The API takes a PIDL, so there is no quoting step to get wrong and a path
// containing spaces, commas or shell metacharacters is handled like any other.
func RevealPath(path string) error {
	// ILCreateFromPathW resolves a relative path against the current working
	// directory, and the shell API expects an item at an absolute location.
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absPtr, err := syscall.UTF16PtrFromString(abs)
	if err != nil {
		return err
	}

	pidl, _, _ := procILCreateFromPathW.Call(uintptr(unsafe.Pointer(absPtr)))
	if pidl == 0 {
		return fmt.Errorf("ILCreateFromPathW failed for %q", abs)
	}
	// The PIDL comes from the shell's private allocator; ILFree is the only
	// correct way to release it. LocalFree or CoTaskMemFree would corrupt the
	// heap.
	defer procILFree.Call(pidl)

	// cidl = 0 with apidl = NULL selects the item the PIDL points at itself:
	// the parent folder opens and the item is highlighted inside it.
	ret, _, _ := procSHOpenFolderAndSelectItems.Call(pidl, 0, 0, 0)
	if ret != 0 {
		return fmt.Errorf("SHOpenFolderAndSelectItems failed for %q: hr=%#x", abs, ret)
	}
	return nil
}