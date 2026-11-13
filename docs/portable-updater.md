# SniShaper 便携式自动更新器设计

## 1. 结论与方案选型

采用 **Go 原生单文件 PE（`CGO_ENABLED=0`）**：标准库 `net/http`、`archive/zip`、`crypto/*` + Win32 API（`syscall.NewLazyDLL`）+ `x/sys/windows/registry`。编译产物只有一个 `SniShaperUpdater.exe`，无 .NET / Java / Python / Node / VC++ 运行库依赖，不拉起任何外部进程。

| 候选方案 | 结论 | 理由 |
| --- | --- | --- |
| Go 原生单 exe | 采用 | 静态链接；HTTP/HTTPS、TLS 根证书、zip 解压、SHA-256、Ed25519 全在标准库；Win32 直接调用；与仓库 Go 1.27 技术栈一致 |
| C++ / MSYS2 + libcurl + miniz | 备选 | 可控但需额外第三方库与工具链，构建体积和审计成本更高 |
| .NET NativeAOT / self-contained | 不采用 | 体积大、发布链复杂；用户明确要求"若用 .NET 必须自包含"，Go 天然满足且更轻 |
| PowerShell / cmd 脚本 + 系统 tar | 禁止 | 正是本需求要消除的依赖（`app/app_update_install_windows.go` 的旧路径就是这么做的） |
| AutoIt / NSIS 更新器 | 不采用 | 仍需自带解压与下载模块，且难以做 Ed25519 校验和 Win32 进程管理 |

**零外部进程论证**：更新器全部 I/O 走 Win32 API 与 Go 标准库。

- 下载：`net/http`（系统 TLS/根证书，不需 curl）
- 解压：`archive/zip`（不需 tar/unzip/7z）
- 注册表读取：`RegOpenKeyExW`/`RegQueryValueExW`（经 `x/sys/windows/registry`，不调 `reg.exe`）
- 提权启动：`ShellExecuteExW(lpVerb="runas")`（不调 `cmd /c start`）
- 进程管理：`CreateToolhelp32Snapshot`/`Process32*`/`OpenProcess`/`QueryFullProcessImageNameW`/`TerminateProcess`
- 提示与交互：`MessageBoxW`、`PostMessageW(WM_CLOSE)`
- 自替换：`MoveFileExW(MOVEFILE_DELAY_UNTIL_REBOOT)`
- 无 `os/exec`、无 `exec.Command`、无 `CreateProcess("cmd")`、无脚本文件

已编译产物：`build/bin/updater/Windows/x64/SniShaperUpdater.exe`，7,366,144 字节（`-trimpath -ldflags "-s -w"`）。

## 2. 架构与流程

```
[更新器原进程] --自身位于安装目录--> 复制到 %TEMP% 并以 --resume state.json 重启为独立进程 --> 原进程退出
[独立更新进程]
  1 读 updater.json + 本地版本（卸载项 DisplayVersion / Package.appxmanifest / version.txt）
  2 HTTPS 取 manifest.json + manifest.json.sig，Ed25519 验签
  3 版本比较（数值段 + 通道 alpha<beta<rc<stable + 预发布序号）
  4 下载 zip 到临时目录（超时/重试/Range 续传/大小校验）
  5 SHA-256 校验包 -> 内置 zip 解压 -> 定位 Inno setup.exe -> 校验其 SHA-256（可选 Authenticode）
  6 MessageBox 提示保存数据 -> WM_CLOSE 礼貌关闭旧进程 -> 超时后询问强制结束
  7 ShellExecuteExW runas 启动安装包（lpParameters 为空，正常向导 UI）-> WaitForSingleObject + GetExitCodeProcess
  8 退出码判定 -> 读注册表核对版本 -> 清理临时目录 -> 按需重启应用
  9 任何失败：日志 + 弹窗 + 保留临时目录，旧版本文件从未被更新器改动
```

状态与日志位置优先序：更新器同目录（`updater.json`、`updater.log`、`updater-work-*`）→ `%TEMP%\snishaper-updater\`。

## 3. 关键模块设计

| 文件 | 职责 |
| --- | --- |
| `updater/main.go` | 流程编排、CLI 参数、工作目录、自我迁移、失败分类与退出码 |
| `updater/config.go` | 配置/清单结构、卸载项读取（HKLM/HKCU × 32/64 视图）、本地版本、版本比较、续跑状态 |
| `updater/download.go` | 内置 HTTPS 客户端、TLS/连接/响应超时、镜像回退、指数退避重试、`.part` + `Range` 断点续传 |
| `updater/verify.go` | SHA-256、Ed25519 清单签名、WinVerifyTrust Authenticode（可选） |
| `updater/zip.go` | 内置 zip 解压、绝对路径/盘符/`..`/反斜杠穿越/符号链接/解压炸弹/条目数防护、安装包定位与 PE 头校验 |
| `updater/winapi.go` | MessageBox、进程快照与路径核对、WM_CLOSE、TerminateProcess、ShellExecuteEx runas、等待与退出码、延迟替换 |
| `updater/log.go` | 同目录/临时目录追加日志，每次写盘 `Sync` |
| `updater/main_stub.go` | 非 Windows 占位，保证 `go build ./...` 在 Linux 也有可编译目标 |
| `updater/*_test.go` | 版本比较、签名、zip 穿越/限额、安装包定位单测 |

CLI：`-config <path>`、`-resume <state.json>`（内部用）、`-check-only`、`-force`。

## 4. 关键代码示例

### 4.1 非静默提权启动 + 等待退出码（`updater/winapi.go`）

```go
func shellExecute(verb, file, params, dir string, wait bool) (uint32, error) {
	v, err := syscall.UTF16PtrFromString(verb)
	if err != nil { return 0, err }
	f, err := syscall.UTF16PtrFromString(file)
	if err != nil { return 0, err }
	p, err := syscall.UTF16PtrFromString(params)
	if err != nil { return 0, err }
	d, err := syscall.UTF16PtrFromString(dir)
	if err != nil { return 0, err }
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
	if !wait { return 0, nil }
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

code, err := shellExecute("runas", setupPath, "", filepath.Dir(setupPath), true)
```

`lpParameters` 传空字符串：`/SILENT`、`/VERYSILENT`、`/SUPPRESSMSGBOXES`、`/NORESTART` 一律不传；`nShow=SW_SHOWNORMAL`，向导、许可页、安装目录页、完成页全部可见。已提权时 `runas` 不弹 UAC，未提权时正常弹出 UAC 对话框；用户拒绝（ERROR_CANCELLED=1223）判为"用户取消"，不绕过提权。安装包自身 `PrivilegesRequired=admin` 也会请求提权，属于双保险。

### 4.2 进程关闭（`updater/winapi.go`）

```go
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

targets, err := snapshotProcesses(c.ProcessNames, installDir, selfPID)
if messageBox(prompt, "SniShaper 更新器", mbOKCancel|mbIconWarning) != idOK {
	return errUserCancelled
}
pids := map[uint32]bool{}
for _, t := range targets { pids[t.PID] = true }
closeWindowsOf(pids)
deadline := time.Now().Add(20 * time.Second)
for time.Now().Before(deadline) {
	remaining, err := snapshotProcesses(c.ProcessNames, installDir, selfPID)
	if err == nil && len(remaining) == 0 { return nil }
	time.Sleep(500 * time.Millisecond)
}
remaining, err = snapshotProcesses(c.ProcessNames, installDir, selfPID)
if err != nil { return err }
if messageBox(forcePrompt, "SniShaper 更新器", mbOKCancel|mbIconWarning) != idOK {
	return errUserCancelled
}
for _, t := range remaining { terminateProcess(t.PID) }
```

`snapshotProcesses` 用 `CreateToolhelp32Snapshot` + `QueryFullProcessImageNameW` 核对完整路径必须落在 `InstallLocation`，避免误杀同名进程；排除自身 PID。先 `WM_CLOSE` 给用户保存数据的机会，20 秒后仍占用才询问强制 `TerminateProcess`。用户任一步点取消即中止更新，旧版本完全不动。

### 4.3 内置解压与安装包定位（`updater/zip.go`）

```go
name := strings.ReplaceAll(f.Name, "\\", "/")
if name == "" || strings.Contains(name, "\x00") {
	return fmt.Errorf("invalid archive entry %q", f.Name)
}
if strings.HasPrefix(name, "/") || (len(name) >= 2 && name[1] == ':') {
	return fmt.Errorf("absolute archive entry %q", f.Name)
}
clean := filepath.Clean(filepath.FromSlash(name))
if clean == ".." || strings.HasPrefix(clean, ".."+sep) || filepath.IsAbs(clean) {
	return fmt.Errorf("archive entry escapes destination: %q", f.Name)
}
if f.Mode()&os.ModeSymlink != 0 {
	return fmt.Errorf("symlink archive entry rejected: %q", f.Name)
}
target := filepath.Join(absDest, clean)
rel, err := filepath.Rel(absDest, target)
if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+sep) {
	return fmt.Errorf("archive entry escapes destination: %q", f.Name)
}
if size := int64(f.UncompressedSize64); size < 0 || total+size > maxBytes {
	return fmt.Errorf("archive exceeds extraction limit of %d bytes", maxBytes)
}
```

`findInstaller` 按 `*Setup.exe` 匹配、排除 `unins*`、校验 MZ 头、按目录深度排序取最浅结果。压缩包必须是普通 zip（deflate/store）；CI 用 `ZipFile` 生成更新包即可，不要打成 7z。

### 4.4 清单签名与哈希（`updater/verify.go`）

```go
if !ed25519.Verify(pub, body, sig) { return fmt.Errorf("manifest signature verification failed") }
```

签名覆盖 `manifest.json` 的原始字节，签名文件 `manifest.json.sig`（base64 文本或裸 64 字节）。公钥是 `updater.json` 中 base64 的 32 字节 raw Ed25519 公钥。**强制要求**：`public_key` 非空时必须同时下载并验证 `.sig`，否则拒绝更新；为空时只记 WARNING（开发调试用）。包与安装包各自再做 SHA-256 校验；`verify_authenticode=true` 时额外用 `WinVerifyTrust(WTD_UI_NONE)` 校验 setup.exe 的 Authenticode 签名。

### 4.5 自我占用处理（`updater/main.go`）

```go
func maybeRelocate(cfg Config, cfgPath, baseDir string, l *Logger) (bool, error) {
	exe, err := os.Executable()
	if err != nil { return false, err }
	installDir := cfg.InstallDir
	if installDir == "" { installDir = readInstalled(cfg).Dir }
	if installDir == "" || !pathWithin(exe, installDir) { return false, nil }
	stage, err := os.MkdirTemp(os.TempDir(), "snishaper-updater-")
	if err != nil { return false, err }
	clone := filepath.Join(stage, filepath.Base(exe))
	if err := copyFile(exe, clone); err != nil { return false, err }
	workDir := filepath.Join(stage, "work")
	if err := os.MkdirAll(workDir, 0755); err != nil { return false, err }
	statePath := filepath.Join(stage, "state.json")
	s := ResumeState{ConfigPath: cfgPath, BaseDir: baseDir, InstallDir: installDir, LogPath: l.Path(), WorkDir: workDir}
	if err := writeResumeState(statePath, s); err != nil { return false, err }
	if _, err := shellExecute("open", clone, `--resume "`+statePath+`"`, stage, false); err != nil { return false, err }
	l.Logf("relocated updater to %s to avoid locking the install directory", clone)
	return true, nil
}
```

更新器位于安装目录时，先把自身复制到 `%TEMP%` 并以 `--resume` 启动独立进程，原进程立即退出，安装目录不再被更新器占用；更新成功后删除 `state.json`，并用 `MoveFileExW(path, NULL, MOVEFILE_DELAY_UNTIL_REBOOT)` 把运行中的临时副本注册为重启后删除（运行中的 exe 无法当场删除）。若清单带 `updater.name` + `sha256`，校验后复制到同目录 `updater.pending.exe`，用 `MoveFileExW(MOVEFILE_REPLACE_EXISTING|MOVEFILE_DELAY_UNTIL_REBOOT)` 注册重启替换（`updater/winapi.go` 的 `scheduleReplacement` / `scheduleDeletion`）。

## 5. 配置与发布清单

`updater/updater.example.json`、`updater/manifest.example.json` 是完整样例。要点：

| 字段 | 说明 |
| --- | --- |
| `manifest_url` | 必须 `https://`；`.sig` 从同 URL 加后缀取 |
| `mirrors` / `mirror_urls` | 镜像前缀（如 `https://gh-proxy.org/`），原站失败后依次重试 |
| `public_key` | 32 字节 raw Ed25519 公钥的 base64 |
| `app_id` | 必须与 `.iss` 的 `AppId` 完全一致；x64/x86/arm64 的 GUID 在 CI 里不同，按架构填 |
| `installer_pattern` / `installer_name` | 定位安装包；清单里的 `installer.name` 优先 |
| `process_names` | 需关闭的进程名，默认 `snishaper.exe` |
| `local_version` | `auto` 时按 卸载项 DisplayVersion → `Package.appxmanifest` → `version.txt` → `0.0.0.0` |
| `verify_authenticode` | CI 未给 setup.exe 做 Authenticode 签名时保持 `false` |
| `restart_after_install` | 默认 `false`；Inno `[Run]` 的 postinstall 已提供启动勾选，避免重复启动 |

发布端已接入 CI（`_release_pipeline.yml` 的 `build-windows` 任务）：

1. 用 Inno Setup 6/7 编译 `Snishaper-<version>-<arch>Setup.exe`。
2. `Build portable updater` 编译三架构 `SniShaperUpdater.exe` 到 `build/bin/updater/Windows/<arch>/`。
3. `Package updater bundles` 调用 `.github/scripts/build-updater-bundle.ps1`，把每个架构的 setup.exe 与 `SniShaperUpdater.exe` 打成普通 zip `snishaper-<version>-windows-<arch>-update.zip`，生成 `updater-manifest-<arch>.json`（version / channel / URL / SHA-256 / size），并把更新器 exe 单独作为一个 release 资产。
4. `Sign updater manifests` 仅在设置 `UPDATER_ED25519_SEED` secret 时运行，调用 `updater/signmanifest` 对每个清单生成 `.sig`（base64）和 `updater-public-key.txt`。seed 是 32 字节 Ed25519 私钥种子的 base64，公钥文件的内容就是 `updater.json` 的 `public_key`。

手工生成 seed 与签名（不依赖 openssl，工具用 Go 标准库）：

```powershell
$seed = [Convert]::ToBase64String([byte[]](1..32 | ForEach-Object { Get-Random -Maximum 256 }))
$env:UPDATER_ED25519_SEED = $seed
go run ./updater/signmanifest -in manifest.json -pub-out updater-public-key.txt
```

私钥只在 CI；公钥随 `updater.json` 分发。校验失败（清单被篡改、哈希不符、签名缺失）直接终止，不启动安装包。`updater.example.json` 里的 `public_key` 是占位串，必须替换成真实公钥；`loadConfig` 会在启动阶段解码校验，占位串直接报错，属于刻意的 fail-closed 设计，不会静默降级为无签名模式。未配置 secret 时 CI 只产出无签名清单，更新器在 `public_key` 为空时记 WARNING 跳过验签。

## 6. Inno Setup 脚本配置

完整示例：`docs/inno-setup-upgrade.iss.example`（可直接改版本号/路径后编译）。关键项：

```ini
[Setup]
AppId={{3F2E4DA1-5C8B-4ECD-BDC4-426A5965F8D4}
AppName=Snishaper
AppVersion=1.30.1
AppPublisher=SnishaperTeam And JetCPPTeam
DefaultDirName={autopf}\Snishaper
DefaultGroupName=Snishaper
PrivilegesRequired=admin
UsePreviousAppDir=yes
CloseApplications=yes
RestartApplications=no
Uninstallable=yes
```

```ini
[Files]
Source: "build\bin\gui\Windows\x64\*"; DestDir: "{app}"; Flags: ignoreversion overwritereadonly restartreplace recursesubdirs createallsubdirs

[InstallDelete]
Type: files; Name: "{app}\legacy-component.exe"

[Registry]
Root: HKLM; Subkey: "Software\Snishaper"; ValueType: string; ValueName: "Version"; ValueData: "1.30.1"; Flags: uninsdeletevalue
Root: HKLM; Subkey: "Software\Snishaper"; ValueType: string; ValueName: "InstallPath"; ValueData: "{app}"; Flags: uninsdeletevalue

[Run]
Filename: "{app}\snishaper.exe"; Description: "{cm:LaunchProgram,Snishaper}"; Flags: nowait postinstall skipifsilent
```

论证（依据 Inno Setup 官方文档）：

- **AppId 稳定即升级**：AppId 决定卸载日志 `unins???.dat` 的追加判定与卸载注册表键名 `{AppId}_is1`。同一架构每次发版不改 AppId，Inno 追加到同一卸载日志，不新建第二个卸载项。CI 现脚本按 x64/x86/arm64 使用三个固定 GUID，符合"同架构稳定"，不要改成随版本变化的值。
- **`ignoreversion` 就是正确的覆盖策略**：Inno 默认会按文件版本信息比较，同版本时**不替换**；`ignoreversion` 的含义是"完全不比较版本信息，无条件替换已存在文件"。因此现脚本里的 `ignoreversion` 不要删。新增 `overwritereadonly`（只读文件不再弹询问）与 `restartreplace`（占用文件注册到重启后替换，需 admin）。
- **同目录策略**：`UsePreviousAppDir=yes`（官方默认 yes）在检测到同 AppId 旧安装时把旧目录作为向导默认值；`DefaultDirName` 保持 `{autopf}\Snishaper` 不变。
- **进程占用**：`CloseApplications=yes` 官方文档默认即为 yes，但显式写出。它借助 Windows Restart Manager 在"Preparing to Install"页列出占用文件的应用，询问用户是否关闭并可在装完重启；因为更新器不传静默参数，这个交互页正常出现。`RestartApplications=no` 避免装完自动拉起旧进程与 `[Run]` 冲突。`CloseApplicationsFilter` 可按需限定关注的文件类型，示例不覆盖默认值。
- **权限与卸载项唯一性**：`PrivilegesRequired=admin`（官方默认 admin）保证写 HKLM 与 Program Files。**必须移除** `PrivilegesRequiredOverridesAllowed=dialog`：允许用户在 admin/lowest 间切换会产生 HKLM 与 HKCU 两套卸载项，是"重复卸载项"的典型根因。
- **注册表**：`[Registry]` 用固定根键/子键，`ValueData` 引用 `{#MyAppVersion}`，每次安装自动更新。卸载项（DisplayName/DisplayVersion/InstallLocation/UninstallString）由 Inno 自己维护，更新器**不写注册表模拟安装**，只用 Win32 API 读取 `DisplayVersion`/`InstallLocation` 做版本核对。
- **旧文件残留**：新版删掉的组件用 `[InstallDelete]` 显式列出；不要写 `{app}\*` 之类的通配删除，会毁掉用户配置和规则数据。
- **非静默**：脚本不设置任何默认静默行为；`[Run]` 的 `skipifsilent` 只是"静默时不启动应用"，在正常向导下仍显示启动勾选框。

### 6.1 CI 脚本检查与修复结论

检查对象：`.github/scripts/create-inno-installer.ps1` 里的 `$issTemplate`。本次已把下表"原缺失"项全部补齐。

| 要求 | 修复前 | 现状 |
| --- | --- | --- |
| 稳定 AppId | 已按架构固定三个 GUID | 保持 |
| AppName/AppVersion/Publisher/DefaultDirName | 已有 | 保持 |
| DefaultGroupName | 缺失 | 已补 `DefaultGroupName={#MyAppName}` |
| CloseApplications / RestartApplications | 缺失（仅依赖默认 yes） | 已补 `CloseApplications=yes`、`RestartApplications=no` |
| Uninstallable | 缺失 | 已补 `Uninstallable=yes` |
| PrivilegesRequired | 缺失，且设了 `PrivilegesRequiredOverridesAllowed=dialog` | 已补 `PrivilegesRequired=admin` 并删除 dialog，消除 HKLM/HKCU 双卸载项 |
| UsePreviousAppDir | 缺失 | 已补 `UsePreviousAppDir=yes` |
| `[Registry]` 版本信息 | 完全缺失 | 已补 `Software\Snishaper` 的 Version / InstallPath / Publisher |
| 文件覆盖 | `Flags: ignoreversion` | 已加 `overwritereadonly restartreplace` |
| 旧文件残留清理 | 缺失 | `[InstallDelete]` 在 `docs/inno-setup-upgrade.iss.example` 给出示例；CI 模板不写通配删除，避免误删用户数据，需按实际废弃清单维护 |
| 默认静默 | 无静默配置；`[Run]` 有 `skipifsilent` | 保持 |

另注：`app/app_update_install_windows.go` 的便携版更新路径仍用 `cmd /c start powershell -File apply-update.ps1`，且脚本内调 `tar`/`7z.exe`；`launchInstaller` 用 `exec.Command` 启动 exe，隐藏窗口、不等待、不提权、不关旧进程。这两条路径**不满足本次硬性约束**，本次新增的独立更新器与它们解耦；若要让应用内置更新也合规，需要把该路径替换为调用本更新器（或直接复用本包），属于后续独立改动。

## 7. 退出码、日志与失败处理

Inno Setup 官方 "Setup Exit Codes"：0 成功；1 初始化失败；2 安装前取消；3 阶段切换致命错误；4 安装过程致命错误；5 安装中取消/中止；6 调试器强制终止；7 Preparing to Install 判定无法继续；8 判定需重启才能继续。**任何非零都表示未完成安装**。

| 情况 | 更新器行为 | 自身退出码 |
| --- | --- | --- |
| 安装成功 | 清理临时目录、核对注册表版本、可选重启应用 | 0 |
| 无更新 / 仅检查 | 只写日志 | 0 |
| 清单/包下载失败 | 日志 + 弹窗 + 保留临时目录 | 10 |
| 签名/哈希/Authenticode 失败 | 不启动安装包，旧版完好 | 11 |
| 解压/找不到安装包 | 不启动安装包 | 12 |
| 用户拒绝提权 | 中止，旧版完好 | 14 |
| 安装包非零退出（1/3/4/6/7） | 记录退出码与含义 | 15 |
| 用户取消（2/5、关闭提示取消） | 静默中止，不弹错误框 | 16 |
| 需要重启（8） | 提示重启后重跑 | 17 |

失败时保留下载包与解压目录，日志写明路径；更新器全程不改动安装目录里的任何旧文件，安装前的所有失败都保证旧版本可运行。安装阶段失败由 Inno 自身处理，更新器记录退出码并保留原始安装包，可直接手动重跑或用旧版安装包回退。日志：`updater.log`（同目录优先，否则 `%TEMP%\snishaper-updater\updater.log`），每行带毫秒时间戳并立即刷盘。

## 8. 构建与部署

普通构建即可，不需要任何额外参数：

```powershell
.\build_windows.ps1 -Build all -Silent
```

每个 Windows GUI 目标构建时都会顺带编译对应架构的 `SniShaperUpdater.exe`、生成 `updater.json`，并把两者放进 `build\bin\gui\Windows\<arch>\`。安装包、便携 zip、MSIX 全部由这个目录打包，所以更新器始终与主程序在一起。

单独编译与校验：

```powershell
$env:GOOS='windows'; $env:CGO_ENABLED='0'
go test ./updater/... -count=1 -v
go vet ./updater/...
go build -trimpath -ldflags "-s -w" -o build/bin/updater/Windows/x64/SniShaperUpdater.exe ./updater
```

CI 侧由 `_release_pipeline.yml` 的 `build-windows` 任务负责三架构编译、更新包打包、清单生成与可选 Ed25519 签名；发布阶段把新资产（更新器 exe、更新包 zip、清单、签名、公钥）加入资产表、上传 pattern 与下载校验，asset 表新增对应用途说明。

### 8.1 GUI 内置更新路径（无 cmd / pwsh / tar / 7z）

`app/app_update_install_windows.go` 重写后，安装版更新不再生成或启动任何脚本：

| 环节 | 实现 |
| --- | --- |
| `InstallUpdateAsset` | 先找同目录 `SniShaperUpdater.exe`；找到就用 `ShellExecuteW("open")` 启动，参数 `-config "<安装目录>\updater.json"` |
| 没有更新器（旧版本首次升级） | `.exe` 资产回退：`ShellExecuteW("runas")` 正常 UI 启动下载的 Inno 安装包，不传任何静默参数 |
| `.zip` / `.7z` 资产且没有更新器 | 返回明确错误，提示从发版页手动下载安装包；不再拉起 `cmd /c start powershell` |
| 主程序退出 | 不主动杀进程：更新器自己做"请保存数据"提示 → `WM_CLOSE` → 必要时询问强制结束 |

`app/app_update_test.go` 断言 `filterUpdateAssets` 排除 `SniShaperUpdater-*.exe` 与 `*-update.zip`，避免 GUI 把更新器或更新包当成安装包下载；`app/app_update_install_windows_test.go` 静态断言安装路径源码里不出现 `powershell`、`cmd`、`tar`、`7z.exe`、`os/exec`。

更新器始终与主程序打包在一起：Windows GUI 构建把 `SniShaperUpdater.exe` 与 `updater.json` 放进 `build\bin\gui\Windows\<arch>\`，Inno 安装包（`Source: "__BINDIR__\*"`）、便携 zip（`CreateFromDirectory`）与 MSIX（`winapp pack`）都从该目录打包，因此三者都自带更新器。配置由 CI 生成（指向 `releases/latest/download/updater-manifest-<arch>.json`、该架构的 `AppId`、签名公钥），本地普通构建生成同名配置，并在设置了 `UPDATER_ED25519_SEED` 时派生公钥。MSIX 里的更新器随包签名，但沙箱应用仍不能在运行时下载并执行新文件，MSIX 的更新由系统商店负责。

### 8.2 发版公告

`publish` 任务在 release 创建、资产校验、贡献者列表补齐之后执行 `Publish release announcement`：

1. `POST https://jetcpp.ccwu.cc/api/admin/session`，body `{email, password}`，取回 `token`。
2. `POST https://jetcpp.ccwu.cc/api/notices`，`Authorization: Bearer <token>`，body 为公告对象。
3. 公告字段：`id = release-<tag>`（幂等，重复发版替换同一条）、`title` / `title_en` / `title_ru`、`body` / `body_en` / `body_ru`（本次更新内容取自 `RELEASE_NOTES.md` 的 Changes 段，三语各一份）、`level`（正式版 `important`，预发布 `info`）、`published_at`、`link` = 具体 release 页面 `https://github.com/<repo>/releases/tag/<tag>`、`link_text`、`pinned = false`。

凭据走仓库 secret，缺失时整个 step 跳过，发版不受影响：

| Secret | 用途 |
| --- | --- |
| `ANNOUNCEMENT_ADMIN_EMAIL` | 公告后台管理员邮箱 |
| `ANNOUNCEMENT_ADMIN_PASSWORD` | 公告后台管理员密码 |
| `UPDATER_ED25519_SEED` | 32 字节 Ed25519 私钥种子（base64）；CI 用它签清单并把公钥写进 `updater.json` |


部署布局（全部便携，无需安装）：

```
SniShaperUpdater.exe
updater.json
updater.log          (运行时生成)
updater-work-*/      (运行时生成，成功后删除)
```

也可把 exe 单独放进安装目录，`updater.json` 与 exe 同目录；此时更新器会自动复制自身到 `%TEMP%` 再继续。CI 已按 `GOARCH=amd64/386/arm64` 分别编译三架构；各架构的 `updater.json` 必须填对应架构的 `app_id`（CI 脚本里三个固定 GUID）。

## 9. 测试清单

| # | 场景 | 操作 | 通过标准 |
| --- | --- | --- | --- |
| 1 | cmd / PowerShell 被禁用 | 组策略禁止 `cmd.exe`、`powershell.exe`、`pwsh.exe`，或临时改名（测试机） | 全流程完成；进程树无 cmd/pwsh |
| 2 | 无外部解压工具 | 移除/改名 `tar.exe`、`7z.exe`、任何 unzip；断网状态下用本地 HTTPS 服务提供 zip | 解压成功且定位到 setup.exe |
| 3 | 无外部运行时 | 干净 Windows（无 .NET 桌面运行时以外的组件、无 Python/Node/Java）、无 VC++ 运行库 | exe 直接启动，无 "缺少 DLL" 提示 |
| 4 | 进程树审计 | Process Explorer / ProcMon 过滤父进程为 `SniShaperUpdater.exe` | 仅出现 `SniShaperUpdater.exe`（临时副本）与 Inno `setup.exe`；无 reg.exe/tar.exe/7z.exe/wmic |
| 5 | 非静默安装 | 正常跑一次完整更新 | 出现许可页、目录页、进度页、完成页；`lpParameters` 为空；无 `/SILENT`/`/VERYSILENT`/`/SUPPRESSMSGBOXES` |
| 6 | UAC 提权 | 标准用户运行；再测管理员已提权运行 | 前者弹 UAC 并正常安装；后者不重复弹；拒绝 UAC 时退出码 14 且旧版完好 |
| 7 | 旧进程占用 | 启动旧版并打开配置窗口，触发更新 | 先弹"保存数据"提示；点取消则中止且旧版仍在运行；点确定则 WM_CLOSE 关闭；顽固进程才询问强制结束 |
| 8 | 文件覆盖与版本 | 旧版 1.30.0 → 新版 1.30.1，检查 exe 时间戳/文件版本、规则文件 | 文件被覆盖，`DisplayVersion=1.30.1`，`InstallLocation` 不变 |
| 9 | 注册表升级无重复项 | 更新前后查 `HKLM\...\Uninstall\{AppId}_is1` 及 WOW6432Node、HKCU | 只有一个卸载项；DisplayName/DisplayVersion/InstallLocation/UninstallString 正确；`Software\Snishaper\Version` 更新 |
| 10 | 升级时权限模式一致 | 连续两次都以 admin 安装 | 不出现 HKLM + HKCU 双卸载项 |
| 11 | 清单签名/哈希 | 篡改 manifest.json、替换 zip、删 `.sig` | 分别被 Ed25519 / SHA-256 / 签名缺失拦截，不启动安装包 |
| 12 | 断点续传 | 下载到 40% 时断网再恢复 | `.part` + `Range` 续传，最终 SHA-256 一致 |
| 13 | 安装取消 | 在 Inno 向导点 Cancel（安装前与安装中各一次） | 退出码 2/5 → 更新器 16；日志；临时目录保留；旧版可运行 |
| 14 | 安装失败 | 让 setup.exe 返回 4（占用/权限错误） | 退出码 15；日志含 Inno 退出码含义；旧版未被更新器改动 |
| 15 | 自我占用 | 把更新器放到 `{app}` 内运行 | 自动复制到 `%TEMP%` 并以 `--resume` 续跑；安装目录无更新器文件锁 |
| 16 | 更新器自更新 | 清单带 `updater.name`/`sha256` | 校验通过后 `updater.pending.exe` + 延迟替换注册成功，日志提示重启生效 |
| 17 | 日志与便携性 | 把整个目录拷到 U 盘/只读父目录 | 只读时回退 `%TEMP%` 写日志；`updater.log` 含全部阶段与错误原因 |
| 18 | 架构匹配 | x64/x86/arm64 各跑一次 | 下载对应架构包，`app_id` 匹配对应 GUID，不串用卸载项 |

## 10. 已知边界

- 更新包必须是普通 zip；7z/rar 需要自带解码库，当前不提供（发布端控制格式即可）。
- `MOVEFILE_DELAY_UNTIL_REBOOT` 需要管理员权限；非提权环境下自更新只记 WARNING，不影响主程序更新。
- `verify_authenticode=true` 要求发布端给 setup.exe 做 Authenticode 签名，否则会拒绝安装；当前 CI 未签，保持 `false`。
- 更新器只读注册表（卸载项），不写任何注册表来模拟安装；注册表更新全部由 Inno 脚本完成。
- 按 `AGENTS.md` 约定，新增 `updater/` 与 `docs/` 文件已同步更新 GitHub Wiki：`Portable-Updater.md`、`EN-Portable-Updater.md` 与 `_Sidebar.md`。
- CI 签名需要 `UPDATER_ED25519_SEED` secret；未配置时只产出无签名清单，更新器在 `public_key` 为空时以 WARNING 跳过验签，不会拒绝更新。
- 安装包内的 `updater.json` 指向 `releases/latest/download/...`，即正式版通道；预发布通道用户需把 `manifest_url` 改成对应 release 的资产地址，并同步 `channel`。
- 公告发布依赖 `ANNOUNCEMENT_ADMIN_EMAIL` / `ANNOUNCEMENT_ADMIN_PASSWORD`；缺失时 CI 跳过该 step，不影响 release 发布。
