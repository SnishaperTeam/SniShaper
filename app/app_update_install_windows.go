//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// installUpdateAsset dispatches installation of a downloaded update asset
// based on its file type (Windows-specific formats).
func (a *App) installUpdateAsset(localPath string) error {
	lower := strings.ToLower(localPath)
	switch {
	case strings.HasSuffix(lower, ".exe"):
		return a.launchInstaller(localPath)
	case strings.HasSuffix(lower, ".7z"):
		return a.installSevenZ(localPath)
	default:
		return fmt.Errorf("unsupported update file type: %s", localPath)
	}
}

func (a *App) launchInstaller(localPath string) error {
	a.appendLog("[update] Launching installer: " + localPath)
	cmd := exec.Command(localPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start installer: %v", err)
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func (a *App) installSevenZ(localPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate application directory: %v", err)
	}
	execDir := filepath.Dir(execPath)
	if !isDirWritable(execDir) {
		return fmt.Errorf("dir_not_writable")
	}
	base := filepath.Join(os.TempDir(), "snishaper-update")
	stage := filepath.Join(base, "stage")
	os.RemoveAll(stage)
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}
	script := filepath.Join(base, "apply-update.ps1")
	content := buildUpdateScript(localPath, stage, execDir, base)
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(content)...)
	if err := os.WriteFile(script, bom, 0644); err != nil {
		return err
	}
	// 用 cmd /c start 启动：让 pwsh 拥有独立的新控制台窗口
	// （直接 exec 启动会继承 GUI 父进程的无效 std 句柄，新窗口收不到输出）
	launcher := exec.Command("cmd", "/c", "start", "SniShaper 更新", "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script)
	launcher.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := launcher.Start(); err != nil {
		return fmt.Errorf("failed to launch updater script: %v", err)
	}
	go func() {
		_ = launcher.Wait()
	}()
	a.appendLog("[update] 7z update script launched (pwsh, visible window)")
	return nil
}

func buildUpdateScript(archive, stage, execDir, base string) string {
	return `$ErrorActionPreference = 'Continue'
$Host.UI.RawUI.WindowTitle = 'SniShaper 便携版更新'
Write-Host ''
Write-Host '========================================' -ForegroundColor Cyan
Write-Host '          SniShaper 便携版更新' -ForegroundColor Cyan
Write-Host '========================================' -ForegroundColor Cyan
Write-Host ''

$archive = '` + archive + `'
$stage = '` + stage + `'
$execDir = '` + execDir + `'
$base = '` + base + `'
$exe = Join-Path $execDir 'snishaper.exe'

Write-Host '[1/5] 正在解压更新包...' -ForegroundColor Yellow
& tar -xf $archive -C $stage | Out-Null
if ($LASTEXITCODE -ne 0 -or -not (Test-Path (Join-Path $stage 'snishaper.exe'))) {
    Write-Host '[错误] 解压更新包失败。' -ForegroundColor Red
    Read-Host '按回车键退出'
    exit 1
}
Write-Host '      解压完成。' -ForegroundColor Green

Write-Host '[2/5] 正在停止旧进程...' -ForegroundColor Yellow
Stop-Process -Name 'snishaper' -Force -ErrorAction SilentlyContinue
for ($i = 0; $i -lt 30; $i++) {
    if (-not (Get-Process -Name 'snishaper' -ErrorAction SilentlyContinue)) { break }
    Start-Sleep -Milliseconds 500
}
Write-Host '      旧进程已退出。' -ForegroundColor Green

Write-Host '[3/5] 正在覆盖程序文件...' -ForegroundColor Yellow
$ok = $false
for ($i = 0; $i -lt 10; $i++) {
    try {
        Copy-Item -Path (Join-Path $stage 'snishaper.exe') -Destination $exe -Force -ErrorAction Stop
        $ok = $true
        break
    } catch { Start-Sleep -Seconds 1 }
}
if (-not $ok) {
    Write-Host '[错误] 覆盖主程序失败，文件可能仍被占用。' -ForegroundColor Red
    Read-Host '按回车键退出'
    exit 2
}
Copy-Item -Path (Join-Path $stage 'rules') -Destination $execDir -Recurse -Force
Write-Host '      文件已更新（保留个人设置）。' -ForegroundColor Green

Write-Host '[4/5] 正在启动新版本...' -ForegroundColor Yellow
for ($i = 0; $i -lt 30; $i++) {
    if (Get-Process -Name 'snishaper' -ErrorAction SilentlyContinue) { break }
    Start-Process -FilePath $exe
    Start-Sleep -Seconds 3
}

Write-Host '[5/5] 清理临时文件...' -ForegroundColor Yellow
Start-Sleep -Seconds 1
Remove-Item -Path $base -Recurse -Force -ErrorAction SilentlyContinue

if (Get-Process -Name 'snishaper' -ErrorAction SilentlyContinue) {
    Write-Host ''
    Write-Host '更新完成，SniShaper 已重新启动。' -ForegroundColor Green
    Start-Sleep -Seconds 2
} else {
    Write-Host ''
    Write-Host '[警告] 未检测到程序进程，请手动打开 SniShaper。' -ForegroundColor Yellow
    Read-Host '按回车键退出'
    exit 3
}
`
}
