//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	portableUpdaterName   = "SniShaperUpdater.exe"
	portableUpdaterConfig = "updater.json"
	swShowNormal          = 1
)

var (
	modShell32        = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

// installUpdateAsset hands the already-downloaded, hash-verified update
// package to the portable updater via -package/-sha256. The updater does no
// downloading itself.
func (a *App) installUpdateAsset(localPath, expectedSHA string) error {
	updaterExe, err := portableUpdaterPath()
	if err != nil {
		return err
	}
	if updaterExe != "" {
		return a.launchPortableUpdater(updaterExe, localPath, expectedSHA)
	}
	if message := archiveInstallError(localPath); message != "" {
		return fmt.Errorf("%s", message)
	}
	return a.launchInstaller(localPath)
}

func archiveInstallError(localPath string) string {
	if strings.HasSuffix(strings.ToLower(localPath), ".exe") {
		return ""
	}
	return "portable updater not found: download the installer from the release page and run it manually"
}

func portableUpdaterPath() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot locate application directory: %v", err)
	}
	return portableUpdaterInDir(filepath.Dir(execPath)), nil
}

func portableUpdaterInDir(dir string) string {
	candidate := filepath.Join(dir, portableUpdaterName)
	if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
		return candidate
	}
	return ""
}

func updaterArgs(configPath, packagePath, sha256 string) string {
	args := fmt.Sprintf(`-package "%s" -sha256 "%s"`, packagePath, sha256)
	if configPath != "" {
		args = fmt.Sprintf(`-config "%s" %s`, configPath, args)
	}
	return args
}

func (a *App) launchPortableUpdater(exe, packagePath, sha256 string) error {
	dir := filepath.Dir(exe)
	config := filepath.Join(dir, portableUpdaterConfig)
	params := updaterArgs("", packagePath, sha256)
	if _, err := os.Stat(config); err == nil {
		params = updaterArgs(config, packagePath, sha256)
	}
	a.appendLog("[update] Launching portable updater: " + exe)
	return shellExecute("open", exe, params, dir)
}

func (a *App) launchInstaller(localPath string) error {
	a.appendLog("[update] Launching installer in normal UI mode: " + localPath)
	return shellExecute("runas", localPath, "", filepath.Dir(localPath))
}

func shellExecute(verb, file, params, dir string) error {
	v, err := syscall.UTF16PtrFromString(verb)
	if err != nil {
		return err
	}
	f, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	p, err := syscall.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	d, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return err
	}
	r, _, callErr := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(f)),
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(d)),
		uintptr(swShowNormal),
	)
	runtime.KeepAlive(v)
	runtime.KeepAlive(f)
	runtime.KeepAlive(p)
	runtime.KeepAlive(d)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW(%s) failed: %v", verb, callErr)
	}
	return nil
}
