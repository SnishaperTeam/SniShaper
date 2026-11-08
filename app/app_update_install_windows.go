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

// updateArchiveFormat identifies how a portable update package is packed.
// zip is the format the updater produces now; 7z is only kept so releases
// published before the switch can still be installed.
type updateArchiveFormat int

const (
	archiveZip updateArchiveFormat = iota
	archive7z
)

// installUpdateAsset dispatches installation of a downloaded update asset
// based on its file type (Windows-specific formats).
func (a *App) installUpdateAsset(localPath string) error {
	lower := strings.ToLower(localPath)
	switch {
	case strings.HasSuffix(lower, ".exe"):
		return a.launchInstaller(localPath)
	case strings.HasSuffix(lower, ".zip"):
		return a.installPortableArchive(localPath, archiveZip)
	case strings.HasSuffix(lower, ".7z"):
		return a.installPortableArchive(localPath, archive7z)
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

// installPortableArchive unpacks a portable package into a staging directory
// and hands the rest of the swap to a detached PowerShell script.
//
// zip is unpacked here with archive/zip: no tar.exe or 7z.exe is needed, which
// is what makes the update path work on a stock Windows 10. The legacy 7z
// format keeps its script-side extraction of tar.exe (bsdtar, shipped since
// Windows 10 1803) with a 7z.exe fallback, because Go has no 7z reader.
func (a *App) installPortableArchive(localPath string, format updateArchiveFormat) error {
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
	os.RemoveAll(base)
	if err := os.MkdirAll(stage, 0755); err != nil {
		return err
	}

	legacyArchive := ""
	switch format {
	case archiveZip:
		a.appendLog("[update] Extracting update package (zip)...")
		if err := extractZip(localPath, stage); err != nil {
			os.RemoveAll(base)
			a.appendLog("[update] zip extraction failed: " + err.Error())
			return fmt.Errorf("extract_failed")
		}
		root := findStageRoot(stage)
		if root == "" {
			os.RemoveAll(base)
			a.appendLog("[update] update package has no payload")
			return fmt.Errorf("bad_archive")
		}
		stage = root
	default:
		legacyArchive = localPath
	}

	script := filepath.Join(base, "apply-update.ps1")
	content := buildUpdateScript(legacyArchive, stage, execDir, base)
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
	a.appendLog("[update] portable update script launched (pwsh, visible window)")
	return nil
}

// psQuoteSingle 将字符串安全嵌入 PowerShell 单引号字符串（' 转义为 ''）
func psQuoteSingle(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// buildUpdateScript renders the detached updater. legacyArchive is the 7z file
// the script has to unpack itself; it is empty when the payload is already
// staged (zip), in which case the script only verifies it.
func buildUpdateScript(legacyArchive, stage, execDir, base string) string {
	legacyArchive = psQuoteSingle(legacyArchive)
	stage = psQuoteSingle(stage)
	execDir = psQuoteSingle(execDir)
	base = psQuoteSingle(base)
	return `$ErrorActionPreference = 'Continue'
$Host.UI.RawUI.WindowTitle = 'SniShaper 便携版更新'
Write-Host ''
Write-Host '========================================' -ForegroundColor Cyan
Write-Host '          SniShaper 便携版更新' -ForegroundColor Cyan
Write-Host '========================================' -ForegroundColor Cyan
Write-Host ''

$archive = '` + legacyArchive + `'
$stage = '` + stage + `'
$execDir = '` + execDir + `'
$base = '` + base + `'
$exe = Join-Path $execDir 'snishaper.exe'
$stagedExe = Join-Path $stage 'snishaper.exe'

if ($archive -ne '') {
    Write-Host '[1/5] 正在解压更新包...' -ForegroundColor Yellow
    & tar -xf $archive -C $stage | Out-Null
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $stagedExe)) {
        $sevenZip = Get-Command '7z.exe' -ErrorAction SilentlyContinue
        if ($sevenZip) {
            Write-Host '      正在改用 7-Zip 解压...' -ForegroundColor Yellow
            & $sevenZip.Source x $archive "-o$stage" -y | Out-Null
        }
    }
    if (-not (Test-Path $stagedExe)) {
        Write-Host '[错误] 解压更新包失败，请手动下载最新版本。' -ForegroundColor Red
        Read-Host '按回车键退出'
        exit 1
    }
} else {
    Write-Host '[1/5] 更新包已解压。' -ForegroundColor Yellow
    if (-not (Test-Path $stagedExe)) {
        Write-Host '[错误] 更新包内容不完整。' -ForegroundColor Red
        Read-Host '按回车键退出'
        exit 1
    }
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
        Copy-Item -LiteralPath $stagedExe -Destination $exe -Force -ErrorAction Stop
        $ok = $true
        break
    } catch { Start-Sleep -Seconds 1 }
}
if (-not $ok) {
    Write-Host '[错误] 覆盖主程序失败，文件可能仍被占用。' -ForegroundColor Red
    Read-Host '按回车键退出'
    exit 2
}
Copy-Item -LiteralPath (Join-Path $stage 'rules') -Destination $execDir -Recurse -Force -ErrorAction SilentlyContinue
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
