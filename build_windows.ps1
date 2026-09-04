param(
    # ---- legacy switches (kept for existing pipelines / Makefile) ----
    [ValidateSet("en", "cn", "ru")]
    [string]$Lang,

    [string[]]$Arch,

    [switch]$InstallDeps,

    [switch]$BuildMsix,

    [switch]$SkipSign,

    [switch]$Cli,

    [switch]$Gtk3,

    [switch]$Silent,

    [string[]]$Build,

    # ---- unified multi-platform target selectors ----
    [string[]]$Platform,

    [string[]]$Type,

    [switch]$All,

    [switch]$DryRun,

    [switch]$CI,

    [switch]$Cross,

    [switch]$Wails,

    [switch]$Help
)

# ===========================================================================
# build_windows.ps1 — SniShaper multi-platform / multi-architecture builder
#                     (Windows host). Bash twin: build.sh
#
# OUTPUT LAYOUT
#   build\bin\cli\Windows\{x64,x86,arm64}\snishaper.exe
#   build\bin\cli\Linux\{x64,arm64}\snishaper
#   build\bin\cli\Darwin\{x64,arm64}\snishaper
#   build\bin\gui\Windows\{x64,x86,arm64}\snishaper.exe
#   build\bin\gui\Linux\{x64,arm64}\SniShaper
#
#   Each target directory also receives the runtime seed folders config\ and
#   rules\. The GUI is never built for Darwin; x86 exists on Windows only.
#
# TARGET MATRIX — 12 artifacts
#   CLI : Windows x64/x86/arm64, Linux x64/arm64, Darwin x64/arm64  = 7
#   GUI : Windows x64/x86/arm64, Linux x64/arm64                    = 5
#
# PARAMETERS
#   -Platform <name>   windows|linux|darwin|all      (repeatable)
#   -Arch <name>       x64|x86|arm64|all             (repeatable)
#   -Type <name>       cli|gui|all                   (repeatable)
#
#   PowerShell binds a named parameter only once and treats a "--name" token
#   as a value, so repeated values use the array syntax:
#       -Type cli,gui                -Platform windows,linux -Arch x64,arm64
#   ("all" works everywhere: -Type all, -Platform all, -Arch all.)
#   build.sh, the Unix twin, accepts the POSIX repeatable form instead:
#       ./build.sh --type cli --type gui --platform windows --arch arm64
#   -All               every platform + architecture (-Type still filters)
#   -DryRun            resolve the target list and exit without building
#   -CI                CI mode: never prompt, never export a cross CC/CXX.
#                      On a Windows runner the GUI is built for Windows only;
#                      linux/arm64 and darwin/arm64 jobs must run natively on
#                      ubuntu-24.04-arm / macos-14 through build.sh.
#   -Cross             local development: allow the Linux GUI to be produced
#                      through WSL with a cross toolchain. Refused under -CI.
#   -Wails             build the GUI through the Wails CLI instead of go build
#   -Silent            never prompt
#   -InstallDeps       run npm install before the frontend build
#   -BuildMsix         package (and sign) an MSIX from the Windows GUI output
#   -SkipSign          package but do not sign (unsigned_*.msix)
#   -Gtk3              build the Linux GUI against GTK3/WebKit2GTK-4.1
#   -Help              show this help
#
# LEGACY PARAMETERS
#   -Build <system>,<mode>,<scope>   e.g. -Build windows,gui,all
#   -Cli  -Arch <arch>  -Lang <en|cn|ru>  -Silent
#
# EXAMPLES
#   .\build_windows.ps1                                # interactive wizard
#   .\build_windows.ps1 -All                           # all 12 targets
#   .\build_windows.ps1 -Type cli -All                 # all 7 CLI targets
#   .\build_windows.ps1 -CI -Platform windows -Arch x64 -Type cli -Type gui
#   .\build_windows.ps1 -CI -Platform windows -Arch arm64 -BuildMsix
#   .\build_windows.ps1 -DryRun -All                   # plan only
# ===========================================================================

if ($Help) {
    $lines = Get-Content -Path $MyInvocation.MyCommand.Path
    $n = $lines.Count
    $i = 0
    while ($i -lt $n -and $lines[$i] -notmatch '^\s*param\s*\(') { $i++ }
    while ($i -lt $n -and $lines[$i] -notmatch '^\s*\)\s*$') { $i++ }
    $i++
    for (; $i -lt $n; $i++) {
        if ($lines[$i] -match '^#\s?(.*)$') { Write-Host $Matches[1] }
        elseif ($lines[$i].Trim() -ne '') { break }
    }
    exit 0
}

# ---------------------------------------------------------------------------
# Console encoding
# ---------------------------------------------------------------------------
try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
    chcp 65001 | Out-Null
} catch {
    # ignore: continue with the default encoding
}

# ---------------------------------------------------------------------------
# Pure helpers (safe to define before the elevation relaunch)
# ---------------------------------------------------------------------------
function Get-NormalizedArch {
    param([string]$Value)
    switch ($Value.ToLower()) {
        "amd64"   { "x64" }
        "x86_64"  { "x64" }
        "x64"     { "x64" }
        "arm64"   { "arm64" }
        "aarch64" { "arm64" }
        "386"     { "x86" }
        "i386"    { "x86" }
        "i686"    { "x86" }
        "x86"     { "x86" }
        "all"     { "all" }
        default   { $null }
    }
}

function Get-NormalizedPlatform {
    param([string]$Value)
    switch ($Value.ToLower()) {
        "windows" { "windows" }
        "win"     { "windows" }
        "linux"   { "linux" }
        "darwin"  { "darwin" }
        "macos"   { "darwin" }
        "mac"     { "darwin" }
        "all"     { "all" }
        default   { $null }
    }
}

function Get-NormalizedType {
    param([string]$Value)
    switch ($Value.ToLower()) {
        "cli"      { "cli" }
        "headless" { "cli" }
        "gui"      { "gui" }
        "desktop"  { "gui" }
        "all"      { "all" }
        default    { $null }
    }
}

# ---------------------------------------------------------------------------
# Administrator elevation (all parameters are forwarded verbatim)
# ---------------------------------------------------------------------------
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "Requesting Administrator privileges..." -ForegroundColor Yellow
    $params = @()
    if ($Build)       { $params += "-Build";       $params += $Build }
    if ($Platform)    { $params += "-Platform";    $params += $Platform }
    if ($Arch)        { $params += "-Arch";        $params += $Arch }
    if ($Type)        { $params += "-Type";        $params += $Type }
    if ($Lang)        { $params += "-Lang";        $params += $Lang }
    if ($InstallDeps) { $params += "-InstallDeps" }
    if ($BuildMsix)   { $params += "-BuildMsix" }
    if ($SkipSign)    { $params += "-SkipSign" }
    if ($Cli)         { $params += "-Cli" }
    if ($Gtk3)        { $params += "-Gtk3" }
    if ($Silent)      { $params += "-Silent" }
    if ($All)         { $params += "-All" }
    if ($DryRun)      { $params += "-DryRun" }
    if ($CI)          { $params += "-CI" }
    if ($Cross)       { $params += "-Cross" }
    if ($Wails)       { $params += "-Wails" }
    $paramStr = $params -join ' '
    Start-Process powershell.exe -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" $paramStr"
    exit
} else {
    try {
        Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force -ErrorAction SilentlyContinue | Out-Null
    } catch {
        # ignore: the policy module may be unavailable in locked-down hosts
    }
}

# ---------------------------------------------------------------------------
# Console encoding
# ---------------------------------------------------------------------------
try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
    chcp 65001 | Out-Null
} catch {
    # ignore: continue with the default encoding
}

$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ProjectRoot

# Kill any running snishaper instances before build
Get-Process -Name "snishaper" -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "[build] Killing snishaper process (PID: $($_.Id))..." -ForegroundColor Yellow
    $_ | Stop-Process -Force
}
Start-Sleep -Milliseconds 500

# ---------------------------------------------------------------------------
# i18n messages (EN / CN / RU)
# ---------------------------------------------------------------------------
$messages = @{
    "LangTitle" = "Please select your language / Select language / Выберите язык"
    "LangOpt1" = "English"
    "LangOpt2" = "Chinese"
    "LangOpt3" = "Russian"
    "LangPrompt" = "Enter your choice (1=English, 2=Chinese, 3=Russian)"

    "EN_MenuTitle" = "       SniShaper Build Menu"
    "EN_DepPrompt" = "Install frontend npm dependencies? [y/N]"
    "EN_MsixPrompt" = "Build the MSIX package? [y/N]"
    "EN_SignPrompt" = "Sign the MSIX package? [Y/n]"
    "EN_TypeTitle" = "Build type (comma separated, empty = all):"
    "EN_TypeOpt1" = "  1) CLI  (headless, cross-platform)"
    "EN_TypeOpt2" = "  2) GUI  (Windows + Linux desktop shell)"
    "EN_TypeOpt3" = "  3) All"
    "EN_PlatformTitle" = "Platform (comma separated, empty = all):"
    "EN_PlatformOpt1" = "  1) Windows"
    "EN_PlatformOpt2" = "  2) Linux"
    "EN_PlatformOpt3" = "  3) Darwin / macOS (CLI only)"
    "EN_PlatformOpt4" = "  4) All"
    "EN_ArchTitle" = "Architecture (comma separated, empty = all):"
    "EN_ArchOpt1" = "  1) x64"
    "EN_ArchOpt2" = "  2) x86 (Windows only)"
    "EN_ArchOpt3" = "  3) arm64"
    "EN_ArchOpt4" = "  4) All"
    "EN_CrossPrompt" = "Allow cross-compilation toolchains for the Linux GUI? [y/N]"
    "EN_ConfirmPrompt" = "Start the build? [Y/n]"
    "EN_NoTargets" = "No valid target matched the selection."
    "EN_PlanTitle" = "Build plan"
    "EN_DryRunNote" = "-DryRun: nothing was built."
    "EN_HostInfo" = "Host: {0}/{1}   CI={2}   cross={3}"
    "EN_FrontEnter" = "[Frontend] Entering frontend directory..."
    "EN_FrontErrDir" = "[Frontend] ERROR: failed to enter the frontend directory!"
    "EN_FrontInstall" = "[Frontend] Installing npm dependencies..."
    "EN_FrontErrInstall" = "[Frontend] ERROR: npm install failed!"
    "EN_FrontBuild" = "[Frontend] Running npm run build..."
    "EN_FrontErrBuild" = "[Frontend] ERROR: npm run build failed!"
    "EN_FrontDone" = "[Frontend] Frontend build completed successfully!"
    "EN_FrontMissing" = "[Frontend] frontend/dist not found: pass -InstallDeps or run npm run build"
    "EN_BackStart" = "[Backend] Starting Go build..."
    "EN_BackInstallDeps" = "[Backend] Installing Go dependencies..."
    "EN_BackErrInstallDeps" = "[Backend] ERROR: go mod download failed!"
    "EN_BackErrBuild" = "[Backend] ERROR: Go build failed!"
    "EN_BackBuildVersion" = "[Backend] Build version: {0}"
    "EN_BackSyncVer" = "[Backend] Syncing version resource from manifest: {0} ..."
    "EN_BackSyncVerDone" = "[Backend] Version resource synced: {0}"
    "EN_BackSyncVerFail" = "[WARNING] go-winres failed, keeping existing version resource"
    "EN_AllDone" = "All selected tasks finished successfully!"
    "EN_Exit" = "Press Enter to exit"
    "EN_Start" = "Starting build process..."
    "EN_SummaryTitle" = "Summary"
    "EN_SummaryBuilt" = "built  : {0}"
    "EN_SummaryFailed" = "failed : {0}"
    "EN_SummarySkipped" = "skipped: {0}"

    "CN_MenuTitle" = "       SniShaper 构建菜单"
    "CN_DepPrompt" = "是否安装前端 npm 依赖？[y/N]"
    "CN_MsixPrompt" = "是否需要构建 MSIX 安装包？[y/N]"
    "CN_SignPrompt" = "是否对 MSIX 进行签名？[Y/n]"
    "CN_TypeTitle" = "请选择构建类型（逗号分隔，留空=全部）："
    "CN_TypeOpt1" = "  1) CLI（headless，跨平台）"
    "CN_TypeOpt2" = "  2) GUI（Windows + Linux 桌面端）"
    "CN_TypeOpt3" = "  3) 全部"
    "CN_PlatformTitle" = "请选择平台（逗号分隔，留空=全部）："
    "CN_PlatformOpt1" = "  1) Windows"
    "CN_PlatformOpt2" = "  2) Linux"
    "CN_PlatformOpt3" = "  3) Darwin / macOS（仅 CLI）"
    "CN_PlatformOpt4" = "  4) 全部"
    "CN_ArchTitle" = "请选择架构（逗号分隔，留空=全部）："
    "CN_ArchOpt1" = "  1) x64"
    "CN_ArchOpt2" = "  2) x86（仅 Windows）"
    "CN_ArchOpt3" = "  3) arm64"
    "CN_ArchOpt4" = "  4) 全部"
    "CN_CrossPrompt" = "是否允许 Linux GUI 使用交叉编译工具链？[y/N]"
    "CN_ConfirmPrompt" = "确认开始构建？[Y/n]"
    "CN_NoTargets" = "没有匹配到任何有效构建目标。"
    "CN_PlanTitle" = "构建计划"
    "CN_DryRunNote" = "-DryRun：未执行任何构建。"
    "CN_HostInfo" = "宿主机: {0}/{1}   CI={2}   cross={3}"
    "CN_FrontEnter" = "[前端] 正在进入 frontend 目录..."
    "CN_FrontErrDir" = "[前端] 错误：无法进入 frontend 目录！"
    "CN_FrontInstall" = "[前端] 正在安装 npm 依赖..."
    "CN_FrontErrInstall" = "[前端] 错误：npm install 失败！"
    "CN_FrontBuild" = "[前端] 正在执行 npm run build..."
    "CN_FrontErrBuild" = "[前端] 错误：npm run build 失败！"
    "CN_FrontDone" = "[前端] 前端构建成功完成！"
    "CN_FrontMissing" = "[前端] 未找到 frontend/dist：请加 -InstallDeps 或先执行 npm run build"
    "CN_BackStart" = "[后端] 正在开始 Go 编译..."
    "CN_BackInstallDeps" = "[后端] 正在安装 Go 依赖..."
    "CN_BackErrInstallDeps" = "[后端] 错误：go mod download 失败！"
    "CN_BackErrBuild" = "[后端] 错误：Go 编译失败！"
    "CN_BackBuildVersion" = "[后端] 构建版本：{0}"
    "CN_BackSyncVer" = "[后端] 正在从 manifest 同步版本资源：{0} ..."
    "CN_BackSyncVerDone" = "[后端] 版本资源已同步：{0}"
    "CN_BackSyncVerFail" = "[警告] go-winres 失败，保留现有版本资源"
    "CN_AllDone" = "所有选定的任务已成功完成！"
    "CN_Exit" = "按回车键退出"
    "CN_Start" = "开始执行构建流程..."
    "CN_SummaryTitle" = "结果汇总"
    "CN_SummaryBuilt" = "成功：{0}"
    "CN_SummaryFailed" = "失败：{0}"
    "CN_SummarySkipped" = "跳过：{0}"

    "RU_MenuTitle" = "       Меню сборки SniShaper"
    "RU_DepPrompt" = "Установить npm зависимости фронтенда? [y/N]"
    "RU_MsixPrompt" = "Создать MSIX-пакет? [y/N]"
    "RU_SignPrompt" = "Подписать MSIX-пакет? [Y/n]"
    "RU_TypeTitle" = "Тип сборки (через запятую, пусто = все):"
    "RU_TypeOpt1" = "  1) CLI  (headless, кроссплатформенный)"
    "RU_TypeOpt2" = "  2) GUI  (Windows + Linux)"
    "RU_TypeOpt3" = "  3) Все"
    "RU_PlatformTitle" = "Платформа (через запятую, пусто = все):"
    "RU_PlatformOpt1" = "  1) Windows"
    "RU_PlatformOpt2" = "  2) Linux"
    "RU_PlatformOpt3" = "  3) Darwin / macOS (только CLI)"
    "RU_PlatformOpt4" = "  4) Все"
    "RU_ArchTitle" = "Архитектура (через запятую, пусто = все):"
    "RU_ArchOpt1" = "  1) x64"
    "RU_ArchOpt2" = "  2) x86 (только Windows)"
    "RU_ArchOpt3" = "  3) arm64"
    "RU_ArchOpt4" = "  4) Все"
    "RU_CrossPrompt" = "Разрешить кросс-тулчейн для Linux GUI? [y/N]"
    "RU_ConfirmPrompt" = "Начать сборку? [Y/n]"
    "RU_NoTargets" = "Не найдено ни одной подходящей цели сборки."
    "RU_PlanTitle" = "План сборки"
    "RU_DryRunNote" = "-DryRun: сборка не выполнялась."
    "RU_HostInfo" = "Хост: {0}/{1}   CI={2}   cross={3}"
    "RU_FrontEnter" = "[Фронтенд] Переход в директорию frontend..."
    "RU_FrontErrDir" = "[Фронтенд] ОШИБКА: не удалось войти в директорию frontend!"
    "RU_FrontInstall" = "[Фронтенд] Установка npm зависимостей..."
    "RU_FrontErrInstall" = "[Фронтенд] ОШИБКА: npm install не удался!"
    "RU_FrontBuild" = "[Фронтенд] Запуск npm run build..."
    "RU_FrontErrBuild" = "[Фронтенд] ОШИБКА: npm run build не удался!"
    "RU_FrontDone" = "[Фронтенд] Сборка фронтенда завершена успешно!"
    "RU_FrontMissing" = "[Фронтенд] frontend/dist не найден: укажите -InstallDeps или выполните npm run build"
    "RU_BackStart" = "[Бэкенд] Начало сборки Go..."
    "RU_BackInstallDeps" = "[Бэкенд] Установка Go зависимостей..."
    "RU_BackErrInstallDeps" = "[Бэкенд] ОШИБКА: go mod download не удался!"
    "RU_BackErrBuild" = "[Бэкенд] ОШИБКА: Сборка Go не удалась!"
    "RU_BackBuildVersion" = "[Бэкенд] Версия сборки: {0}"
    "RU_BackSyncVer" = "[Бэкенд] Синхронизация ресурса версии из manifest: {0} ..."
    "RU_BackSyncVerDone" = "[Бэкенд] Ресурс версии синхронизирован: {0}"
    "RU_BackSyncVerFail" = "[ПРЕДУПРЕЖДЕНИЕ] go-winres не удался, сохраняем текущий ресурс версии"
    "RU_AllDone" = "Все выбранные задачи успешно завершены!"
    "RU_Exit" = "Нажмите Enter для выхода"
    "RU_Start" = "Начало сборки..."
    "RU_SummaryTitle" = "Итог"
    "RU_SummaryBuilt" = "успешно  : {0}"
    "RU_SummaryFailed" = "ошибки   : {0}"
    "RU_SummarySkipped" = "пропущено: {0}"
}

# ---------------------------------------------------------------------------
# Target matrix helpers
# ---------------------------------------------------------------------------
$AllPlatforms = @("windows", "linux", "darwin")
$AllTypes = @("cli", "gui")

function Get-ArchsForPlatform {
    param([string]$Name)
    switch ($Name) {
        "windows" { @("x64", "x86", "arm64") }
        "linux"   { @("x64", "arm64") }
        "darwin"  { @("x64", "arm64") }
        default   { @() }
    }
}

function Test-GuiPlatformSupported {
    param([string]$Name)
    return ($Name -eq "windows" -or $Name -eq "linux")
}

function ConvertTo-GoArch {
    param([string]$Name)
    switch ($Name) {
        "x64"   { "amd64" }
        "arm64" { "arm64" }
        "x86"   { "386" }
        default { $Name }
    }
}

function Get-PlatformDir {
    param([string]$Name)
    switch ($Name) {
        "windows" { "Windows" }
        "linux"   { "Linux" }
        "darwin"  { "Darwin" }
        default   { $Name }
    }
}

function Test-ValidCombo {
    param([string]$TargetType, [string]$TargetPlatform, [string]$TargetArch)
    if ((Get-ArchsForPlatform -Name $TargetPlatform) -notcontains $TargetArch) { return $false }
    if ($TargetType -eq "gui" -and -not (Test-GuiPlatformSupported -Name $TargetPlatform)) { return $false }
    return $true
}

# Get-ValidCombinations — the canonical 12 (type, platform, arch) combinations
function Get-ValidCombinations {
    $result = @()
    foreach ($t in $AllTypes) {
        foreach ($p in $AllPlatforms) {
            foreach ($a in (Get-ArchsForPlatform -Name $p)) {
                if (Test-ValidCombo -TargetType $t -TargetPlatform $p -TargetArch $a) {
                    $result += [pscustomobject]@{ Type = $t; Platform = $p; Arch = $a }
                }
            }
        }
    }
    return $result
}

# ---------------------------------------------------------------------------
# WSL architecture probe (cached): the Linux GUI is built inside WSL, so the
# guest architecture decides which Linux target can be produced natively.
# ---------------------------------------------------------------------------
$script:WslArchCache = $null

function Get-WslArch {
    if ($null -ne $script:WslArchCache) { return $script:WslArchCache }
    $script:WslArchCache = "unknown"
    if (Get-Command wsl.exe -ErrorAction SilentlyContinue) {
        try {
            $machine = (& wsl.exe uname -m 2>$null | Select-Object -First 1)
            if ($machine) {
                $machine = ($machine -replace "[^a-zA-Z0-9_]", "")
                if ($machine -eq "aarch64" -or $machine -eq "arm64") { $script:WslArchCache = "arm64" }
                elseif ($machine -eq "x86_64" -or $machine -eq "amd64") { $script:WslArchCache = "x64" }
            }
        } catch {
            $script:WslArchCache = "unknown"
        }
    }
    return $script:WslArchCache
}

function Get-RelVersion {
    param([string]$ManifestPath)
    $content = Get-Content -Raw $ManifestPath
    $relVersion = ""
    $relChannel = ""
    if ($content -match '<rel:Version>([^<]+)</rel:Version>') { $relVersion = $Matches[1] }
    if ($content -match '<rel:ReleaseChannel>([^<]+)</rel:ReleaseChannel>') { $relChannel = $Matches[1] }
    if ($relVersion) {
        if ($relChannel -and $relChannel -ne 'stable' -and $relChannel -ne 'release' -and $relChannel -ne 'official') {
            return "$relVersion-$relChannel"
        }
        return $relVersion
    }
    [xml]$m = Get-Content $ManifestPath
    $ver = $m.Package.Identity.Version
    $parts = $ver.Split('.')
    if ($parts.Count -eq 4 -and $parts[3] -eq '0') { return ($parts[0..2] -join '.') }
    return $ver
}

# ---------------------------------------------------------------------------
# Host detection
# ---------------------------------------------------------------------------
$HostOs = "windows"
$procArch = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) { $procArch = $env:PROCESSOR_ARCHITEW6432 }
switch ($procArch) {
    "AMD64" { $HostArch = "x64" }
    "ARM64" { $HostArch = "arm64" }
    "x86"   { $HostArch = "x86" }
    default { $HostArch = "x64" }
}

# ---------------------------------------------------------------------------
# Parameter normalisation
# ---------------------------------------------------------------------------
$PlatformSel = @()
$ArchSel = @()
$TypeSel = @()
$BuildFrontend = $true
$BuildBackend = $true
$SelectionGiven = $false

foreach ($v in $Platform) {
    $n = Get-NormalizedPlatform -Value $v
    if (-not $n) { Write-Host "[ERROR] unknown platform: $v" -ForegroundColor Red; exit 1 }
    if ($PlatformSel -notcontains $n) { $PlatformSel += $n }
    $SelectionGiven = $true
}
foreach ($v in $Arch) {
    $n = Get-NormalizedArch -Value $v
    if (-not $n) { Write-Host "[ERROR] unknown architecture: $v" -ForegroundColor Red; exit 1 }
    if ($ArchSel -notcontains $n) { $ArchSel += $n }
    $SelectionGiven = $true
}
foreach ($v in $Type) {
    $n = Get-NormalizedType -Value $v
    if (-not $n) { Write-Host "[ERROR] unknown type: $v" -ForegroundColor Red; exit 1 }
    if ($TypeSel -notcontains $n) { $TypeSel += $n }
    $SelectionGiven = $true
}
if ($All) { $SelectionGiven = $true }

# ---- legacy -Build <system>,<mode>,<scope> ----
if ($Build -and $Build.Count -eq 1 -and $Build[0] -match ',') {
    # powershell.exe -File passes "windows,gui,all" as a single token
    $Build = $Build[0] -split ','
}
if ($Build -and $Build.Count -gt 0) {
    $SelectionGiven = $true
    $sys = if ($Build.Count -ge 1) { $Build[0] } else { "windows" }
    $mode = if ($Build.Count -ge 2) { $Build[1] } else { "gui" }
    $scope = if ($Build.Count -ge 3) { $Build[2] } else { "all" }

    $n = Get-NormalizedPlatform -Value $sys
    if (-not $n) { Write-Host "[ERROR] invalid -Build system: $sys" -ForegroundColor Red; exit 1 }
    $PlatformSel = @($n)

    $n = Get-NormalizedType -Value $mode
    if (-not $n) { Write-Host "[ERROR] invalid -Build mode: $mode" -ForegroundColor Red; exit 1 }
    $TypeSel = @($n)

    switch ($scope.ToLower()) {
        "frontend" { $BuildFrontend = $true;  $BuildBackend = $false }
        "backend"  { $BuildFrontend = $false; $BuildBackend = $true }
        "all"      { $BuildFrontend = $true;  $BuildBackend = $true }
        default    { Write-Host "[ERROR] invalid -Build scope: $scope" -ForegroundColor Red; exit 1 }
    }
}

if ($Cli) {
    $TypeSel = @("cli")
    if ($PlatformSel.Count -eq 0) { $PlatformSel = @("all") }
    $SelectionGiven = $true
}

if ($All) {
    $PlatformSel = @("all")
    $ArchSel = @("all")
    if ($TypeSel.Count -eq 0) { $TypeSel = @("all") }
}

# ---- -DryRun is always non-interactive: show the plan for the full matrix ----
if ($DryRun) { $SelectionGiven = $true }

# ---- CI mode: no prompts, no cross toolchains, no WSL delegation ----
if ($CI) {
    $Silent = $true
    $Cross = $false
}

# ---------------------------------------------------------------------------
# Language
# ---------------------------------------------------------------------------
if ($Lang) {
    $lang = $Lang.ToUpper()
} elseif (-not $Silent -and -not $SelectionGiven) {
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host $messages["LangTitle"] -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "1. $($messages['LangOpt1'])"
    Write-Host "2. $($messages['LangOpt2'])"
    Write-Host "3. $($messages['LangOpt3'])"
    Write-Host ""
    $langChoice = Read-Host $messages["LangPrompt"]
    if ($langChoice -eq "2") { $lang = "CN" }
    elseif ($langChoice -eq "3") { $lang = "RU" }
    else { $lang = "EN" }
} else {
    $lang = "EN"
}

function msg {
    param([string]$Key, [string]$Arg0 = "", [string]$Arg1 = "", [string]$Arg2 = "", [string]$Arg3 = "")
    $val = $messages["$($lang)_$Key"]
    if (-not $val) { return $Key }
    return ($val -replace '\{0\}', $Arg0 -replace '\{1\}', $Arg1 -replace '\{2\}', $Arg2 -replace '\{3\}', $Arg3)
}

# ---------------------------------------------------------------------------
# Target resolution
# ---------------------------------------------------------------------------
function Resolve-Targets {
    $targets = @()
    foreach ($t in $AllTypes) {
        if ($TypeSel.Count -gt 0 -and $TypeSel -notcontains "all" -and $TypeSel -notcontains $t) { continue }
        foreach ($p in $AllPlatforms) {
            if ($PlatformSel.Count -gt 0 -and $PlatformSel -notcontains "all" -and $PlatformSel -notcontains $p) { continue }
            foreach ($a in (Get-ArchsForPlatform -Name $p)) {
                if ($ArchSel.Count -gt 0 -and $ArchSel -notcontains "all" -and $ArchSel -notcontains $a) { continue }

                $state = "build"
                $note = ""

                if (-not (Test-ValidCombo -TargetType $t -TargetPlatform $p -TargetArch $a)) {
                    $state = "skip"
                    $note = "not part of the 12-target matrix (GUI is Windows/Linux only)"
                } elseif ($t -eq "gui" -and $p -ne $HostOs) {
                    if ($CI) {
                        $state = "skip"
                        $note = "GUI needs a native $p runner (this job runs on $HostOs)"
                    } elseif (-not (Get-Command wsl.exe -ErrorAction SilentlyContinue)) {
                        $state = "skip"
                        $note = "GUI needs a native $p host; WSL is not installed"
                    } else {
                        $wslArch = Get-WslArch
                        if ($wslArch -ne $a -and -not $Cross) {
                            $state = "skip"
                            $note = "WSL reports '$wslArch'; GUI $p/$a needs a native $a runner (or -Cross)"
                        } else {
                            $state = "build"
                            $note = "delegated to WSL (native Linux build)"
                        }
                    }
                } else {
                    if ($HostOs -eq $p -and $HostArch -eq $a) { $note = "native host build" }
                    else { $note = "go cross build, CGO_ENABLED=0, no cross toolchain" }
                }

                $outFile = "snishaper"
                if ($p -eq "windows") { $outFile = "snishaper.exe" }
                elseif ($t -eq "gui" -and $p -eq "linux") { $outFile = "SniShaper" }

                $targets += [pscustomobject]@{
                    Type     = $t
                    Platform = $p
                    Arch     = $a
                    GoOs     = $p
                    GoArch   = (ConvertTo-GoArch -Name $a)
                    State    = $state
                    Note     = $note
                    OutDir   = (Join-Path (Join-Path (Join-Path "build\bin" $t) (Get-PlatformDir -Name $p)) $a)
                    OutFile  = $outFile
                }
            }
        }
    }
    return $targets
}

function Show-Plan {
    param($Targets)
    Write-Host ""
    Write-Host (msg -Key "PlanTitle")
    Write-Host "=============================================================================="
    Write-Host ("{0,-32} {1,-8} {2}" -f "TARGET", "STATE", "NOTE")
    Write-Host "------------------------------------------------------------------------------"
    foreach ($t in $Targets) {
        $label = "$($t.Type.ToUpper()) $(Get-PlatformDir -Name $t.Platform)/$($t.Arch)"
        Write-Host ("{0,-32} {1,-8} {2}" -f $label, $t.State, $t.Note)
    }
    Write-Host "=============================================================================="
}

# ---------------------------------------------------------------------------
# Interactive wizard
# ---------------------------------------------------------------------------
if (-not $SelectionGiven -and -not $Silent) {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host (msg -Key "MenuTitle") -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host ""

    if (-not $PSBoundParameters.ContainsKey('InstallDeps')) {
        $depInput = Read-Host (msg -Key "DepPrompt")
        if ($depInput -eq "Y" -or $depInput -eq "y") { $InstallDeps = $true }
    }
    if (-not $PSBoundParameters.ContainsKey('BuildMsix')) {
        $msixInput = Read-Host (msg -Key "MsixPrompt")
        if ($msixInput -eq "Y" -or $msixInput -eq "y") {
            $BuildMsix = $true
            if (-not $PSBoundParameters.ContainsKey('SkipSign')) {
                $signInput = Read-Host (msg -Key "SignPrompt")
                if ($signInput -eq "N" -or $signInput -eq "n") { $SkipSign = $true } else { $SkipSign = $false }
            }
        }
    }

    Write-Host ""
    Write-Host (msg -Key "TypeTitle") -ForegroundColor Yellow
    Write-Host (msg -Key "TypeOpt1")
    Write-Host (msg -Key "TypeOpt2")
    Write-Host (msg -Key "TypeOpt3")
    $ans = Read-Host ">"
    if ($ans -eq "1") { $TypeSel = @("cli") }
    elseif ($ans -eq "2") { $TypeSel = @("gui") }
    else { $TypeSel = @("cli", "gui") }

    Write-Host ""
    Write-Host (msg -Key "PlatformTitle") -ForegroundColor Yellow
    Write-Host (msg -Key "PlatformOpt1")
    Write-Host (msg -Key "PlatformOpt2")
    Write-Host (msg -Key "PlatformOpt3")
    Write-Host (msg -Key "PlatformOpt4")
    $ans = Read-Host ">"
    if ($ans -eq "1") { $PlatformSel = @("windows") }
    elseif ($ans -eq "2") { $PlatformSel = @("linux") }
    elseif ($ans -eq "3") { $PlatformSel = @("darwin") }
    else { $PlatformSel = @("windows", "linux", "darwin") }

    Write-Host ""
    Write-Host (msg -Key "ArchTitle") -ForegroundColor Yellow
    Write-Host (msg -Key "ArchOpt1")
    Write-Host (msg -Key "ArchOpt2")
    Write-Host (msg -Key "ArchOpt3")
    Write-Host (msg -Key "ArchOpt4")
    $ans = Read-Host ">"
    if ($ans -eq "1") { $ArchSel = @("x64") }
    elseif ($ans -eq "2") { $ArchSel = @("x86") }
    elseif ($ans -eq "3") { $ArchSel = @("arm64") }
    else { $ArchSel = @("x64", "x86", "arm64") }

    $crossInput = Read-Host (msg -Key "CrossPrompt")
    if ($crossInput -eq "Y" -or $crossInput -eq "y") { $Cross = $true } else { $Cross = $false }

    Show-Plan -Targets (Resolve-Targets)

    $ok = Read-Host (msg -Key "ConfirmPrompt")
    if ($ok -eq "N" -or $ok -eq "n") {
        Write-Host "aborted." -ForegroundColor Yellow
        exit 0
    }
}

$Targets = Resolve-Targets
if (-not $Targets -or $Targets.Count -eq 0) {
    Write-Host (msg -Key "NoTargets") -ForegroundColor Red
    exit 1
}

# ---------------------------------------------------------------------------
# Header
# ---------------------------------------------------------------------------
$ManifestPath = Join-Path $ProjectRoot "Package.appxmanifest"
$buildVersion = Get-RelVersion -ManifestPath $ManifestPath
$relChannel = ""
$manifestContent = Get-Content -Raw $ManifestPath
if ($manifestContent -match '<rel:ReleaseChannel>([^<]+)</rel:ReleaseChannel>') { $relChannel = $Matches[1].Trim() }

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host " SniShaper build" -ForegroundColor Cyan
Write-Host (" " + (msg -Key "HostInfo" -Arg0 $HostOs -Arg1 $HostArch -Arg2 $CI -Arg3 $Cross)) -ForegroundColor Cyan
Write-Host (" Version : $buildVersion (channel: $relChannel)") -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

Show-Plan -Targets $Targets

if ($DryRun) {
    Write-Host ""
    Write-Host (msg -Key "DryRunNote") -ForegroundColor Yellow
    exit 0
}

# ---------------------------------------------------------------------------
# Frontend build (shared by every GUI target)
# ---------------------------------------------------------------------------
$needFrontend = @($Targets | Where-Object { $_.Type -eq "gui" -and $_.State -eq "build" }).Count -gt 0

function Invoke-FrontendBuild {
    if (-not $BuildFrontend) {
        if (-not (Test-Path (Join-Path $ProjectRoot "frontend\dist\index.html"))) {
            Write-Host (msg -Key "FrontMissing") -ForegroundColor Red
            exit 1
        }
        return
    }

    Write-Host (msg -Key "FrontEnter") -ForegroundColor Green
    try {
        npm --version | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "npm not found" }
    } catch {
        Write-Host "[ERROR] npm is not installed or not in PATH. Please install Node.js first." -ForegroundColor Red
        exit 1
    }

    $FrontendPath = Join-Path $ProjectRoot "frontend"
    if (-not (Test-Path $FrontendPath -PathType Container)) {
        Write-Host "[ERROR] Cannot find frontend directory: $FrontendPath" -ForegroundColor Red
        exit 1
    }

    try { Set-Location $FrontendPath } catch {
        Write-Host (msg -Key "FrontErrDir") -ForegroundColor Red
        exit 1
    }

    if ($InstallDeps) {
        Write-Host (msg -Key "FrontInstall") -ForegroundColor Green
        npm install
        if ($LASTEXITCODE -ne 0) {
            Write-Host (msg -Key "FrontErrInstall") -ForegroundColor Red
            Set-Location $ProjectRoot
            exit 1
        }
    }

    Write-Host (msg -Key "FrontBuild") -ForegroundColor Green
    npm run build
    if ($LASTEXITCODE -ne 0) {
        Write-Host (msg -Key "FrontErrBuild") -ForegroundColor Red
        Set-Location $ProjectRoot
        exit 1
    }

    Write-Host (msg -Key "FrontDone") -ForegroundColor Green
    Set-Location $ProjectRoot
    Write-Host ""
}

# ---------------------------------------------------------------------------
# Windows version resource (.syso) sync — per target architecture
# ---------------------------------------------------------------------------
function Sync-VersionResource {
    param([string]$TargetGoArch)

    # Regenerate the version resource for the exact target architecture right
    # before linking. go-winres runs as a host binary but can emit any arch
    # via --arch, so this works for amd64/386/arm64 on any host. The output is
    # named rsrc_windows_<arch>.syso, which cmd/go only links for that
    # specific Windows target; there is deliberately no bare .syso in the
    # repository, so non-Windows builds never see a Windows resource object.
    # The generated file is removed again after the build (see
    # Invoke-BuildTarget) so the working tree stays clean.
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    $WinResPath = Join-Path $ProjectRoot "winres\winres.json"
    $GenSyso = Join-Path $ProjectRoot ("rsrc_windows_" + $TargetGoArch + ".syso")
    if (-not (Test-Path $WinResPath)) { return }

    $WinResBackup = Get-Content -Raw $WinResPath -Encoding UTF8
    $SysoBackup = $null
    if (Test-Path $GenSyso) { $SysoBackup = [System.IO.File]::ReadAllBytes($GenSyso) }

    try {
        $winres = $WinResBackup | ConvertFrom-Json
        [xml]$wm = Get-Content $ManifestPath
        $mainVer = $wm.Package.Identity.Version
        $winres.RT_VERSION.'#1'.'0000'.fixed.file_version = $mainVer
        $winres.RT_VERSION.'#1'.'0000'.fixed.product_version = $mainVer
        foreach ($langKey in $winres.RT_VERSION.'#1'.'0000'.info.PSObject.Properties.Name) {
            $winres.RT_VERSION.'#1'.'0000'.info.$langKey.FileVersion = $mainVer
            $winres.RT_VERSION.'#1'.'0000'.info.$langKey.ProductVersion = $mainVer
        }
        [System.IO.File]::WriteAllText($WinResPath, ($winres | ConvertTo-Json -Depth 10), $utf8NoBom)
        Write-Host (msg -Key "BackSyncVer" -Arg0 $mainVer) -ForegroundColor Green

        $WinResTmp = Join-Path $env:TEMP ("snishaper-winres-" + $PID + "-" + $TargetGoArch)
        New-Item -ItemType Directory -Path $WinResTmp -Force | Out-Null
        if (Get-Command go-winres -ErrorAction SilentlyContinue) {
            go-winres make --in $WinResPath --arch $TargetGoArch --out (Join-Path $WinResTmp "rsrc")
            $rc = $LASTEXITCODE
        } else {
            go run github.com/tc-hib/go-winres@latest make --in $WinResPath --arch $TargetGoArch --out (Join-Path $WinResTmp "rsrc")
            $rc = $LASTEXITCODE
        }
        if ($rc -ne 0) {
            Write-Host (msg -Key "BackSyncVerFail") -ForegroundColor Yellow
        } else {
            $gen = Get-ChildItem -Path $WinResTmp -Filter ("rsrc_windows_" + $TargetGoArch + ".syso") -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($gen) {
                Copy-Item $gen.FullName $GenSyso -Force
                $script:SysoTargetPath = $GenSyso
                $script:SysoTargetBackup = $SysoBackup
                Write-Host (msg -Key "BackSyncVerDone" -Arg0 $mainVer) -ForegroundColor Green
            } else {
                Write-Host (msg -Key "BackSyncVerFail") -ForegroundColor Yellow
            }
        }
        Remove-Item -Path $WinResTmp -Recurse -Force -ErrorAction SilentlyContinue
    } finally {
        if ($null -ne $WinResBackup) { [System.IO.File]::WriteAllText($WinResPath, $WinResBackup, $utf8NoBom) }
    }
}

# ---------------------------------------------------------------------------
# WSL delegation for the Linux GUI
# ---------------------------------------------------------------------------
function Invoke-WslLinuxGui {
    param($Target)

    $wslArch = "unknown"
    try {
        $machine = (& wsl.exe uname -m 2>$null | Select-Object -First 1)
        if ($machine) {
            $machine = ($machine -replace "[^a-zA-Z0-9_]", "")
            if ($machine -eq "aarch64" -or $machine -eq "arm64") { $wslArch = "arm64" }
            elseif ($machine -eq "x86_64" -or $machine -eq "amd64") { $wslArch = "x64" }
        }
    } catch {
        $wslArch = "unknown"
    }

    if ($wslArch -ne $Target.Arch -and -not $Cross) {
        Write-Host "[SKIP]  GUI $(Get-PlatformDir -Name $Target.Platform)/$($Target.Arch) - WSL reports '$wslArch'; use a native runner or pass -Cross" -ForegroundColor Yellow
        return $false
    }

    $shArgs = @("--ci", "--platform", "linux", "--arch", $Target.Arch, "--type", "gui")
    if ($Gtk3) { $shArgs += "--gtk3" }
    if ($InstallDeps) { $shArgs += "--install-deps" }
    if (-not $BuildFrontend) { $shArgs += @("--build", "linux,gui,backend") }

    Write-Host ""
    Write-Host "[BUILD] GUI $(Get-PlatformDir -Name $Target.Platform) $($Target.Arch) -> WSL: ./build.sh $($shArgs -join ' ')" -ForegroundColor Green
    & wsl.exe --cd $ProjectRoot bash -lc "./build.sh $($shArgs -join ' ')"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[BUILD] FAILED: Linux GUI via WSL (exit $LASTEXITCODE)" -ForegroundColor Red
        return $false
    }
    Write-Host "[BUILD] OK: build\bin\gui\$(Get-PlatformDir -Name $Target.Platform)\$($Target.Arch)" -ForegroundColor Green
    return $true
}

# ---------------------------------------------------------------------------
# Single target build
# ---------------------------------------------------------------------------
function Invoke-BuildTarget {
    param($Target)

    $outDir = Join-Path $ProjectRoot $Target.OutDir
    $out = Join-Path $outDir $Target.OutFile

    Write-Host ""
    Write-Host "[BUILD] $($Target.Type.ToUpper()) $(Get-PlatformDir -Name $Target.Platform) $($Target.Arch) ($($Target.GoOs)/$($Target.GoArch)) -> $($Target.OutDir)\$($Target.OutFile)" -ForegroundColor Green

    New-Item -ItemType Directory -Force -Path $outDir | Out-Null

    if ($Target.Type -eq "gui" -and $Target.GoOs -eq "linux") {
        return (Invoke-WslLinuxGui -Target $Target)
    }

    $ldflags = "-s -w"
    if ($Target.Type -eq "gui" -and $Target.GoOs -eq "windows") { $ldflags += " -H windowsgui" }
    if ($buildVersion) { $ldflags += " -X snishaper/app.buildVersion=$buildVersion" }
    if ($relChannel) { $ldflags += " -X snishaper/app.buildChannel=$relChannel" }

    $rc = 0
    try {
        if ($Target.Type -eq "cli") {
            $env:GOOS = $Target.GoOs
            $env:GOARCH = $Target.GoArch
            $env:CGO_ENABLED = "0"
            go build -tags "with_gvisor headless" -ldflags="$ldflags" -o $out ./cli
            $rc = $LASTEXITCODE
        } else {
            if ($Target.GoOs -eq "windows") { Sync-VersionResource -TargetGoArch $Target.GoArch }
            $env:GOOS = $Target.GoOs
            $env:GOARCH = $Target.GoArch
            $env:CGO_ENABLED = "0"
            if ($Wails) {
                wails build -platform "$($Target.GoOs)/$($Target.GoArch)" -o $out
                $rc = $LASTEXITCODE
            } else {
                go build -tags "with_gvisor" -ldflags="$ldflags" -o $out .
                $rc = $LASTEXITCODE
            }
        }
    } finally {
        Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
        if ($script:SysoTargetBackup -and $script:SysoTargetPath) {
            [System.IO.File]::WriteAllBytes($script:SysoTargetPath, $script:SysoTargetBackup)
        } elseif ($script:SysoTargetPath -and (Test-Path $script:SysoTargetPath)) {
            Remove-Item $script:SysoTargetPath -Force -ErrorAction SilentlyContinue
        }
        $script:SysoTargetPath = $null
        $script:SysoTargetBackup = $null
    }

    if ($rc -ne 0) {
        Write-Host (msg -Key "BackErrBuild") -ForegroundColor Red
        return $false
    }

    if (Test-Path (Join-Path $ProjectRoot "config")) {
        Copy-Item -Path (Join-Path $ProjectRoot "config") -Destination (Join-Path $outDir "config") -Recurse -Force
    }
    if (Test-Path (Join-Path $ProjectRoot "rules")) {
        Copy-Item -Path (Join-Path $ProjectRoot "rules") -Destination (Join-Path $outDir "rules") -Recurse -Force
    }

    Write-Host "[BUILD] OK: $($Target.OutDir)\$($Target.OutFile)" -ForegroundColor Green
    return $true
}

# ---------------------------------------------------------------------------
# Main build loop
# ---------------------------------------------------------------------------
Write-Host ""
Write-Host (msg -Key "Start") -ForegroundColor Green

try {
    go version | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Go not found" }
} catch {
    Write-Host "[ERROR] Go is not installed or not in PATH." -ForegroundColor Red
    exit 1
}

if (-not $BuildBackend) {
    Invoke-FrontendBuild
    Write-Host ""
    Write-Host (msg -Key "AllDone") -ForegroundColor Cyan
    exit 0
}

Write-Host (msg -Key "BackStart") -ForegroundColor Green
Write-Host (msg -Key "BackInstallDeps") -ForegroundColor Green
go mod download
if ($LASTEXITCODE -ne 0) {
    Write-Host (msg -Key "BackErrInstallDeps") -ForegroundColor Red
    exit 1
}
Write-Host (msg -Key "BackBuildVersion" -Arg0 $buildVersion) -ForegroundColor Green

if ($needFrontend) { Invoke-FrontendBuild }

$builtOk = 0
$builtFail = 0
$builtSkip = 0

foreach ($t in $Targets) {
    if ($t.State -ne "build") {
        Write-Host "[SKIP]  $($t.Type.ToUpper()) $(Get-PlatformDir -Name $t.Platform)/$($t.Arch) - $($t.Note)" -ForegroundColor Yellow
        $builtSkip++
        continue
    }
    if (Invoke-BuildTarget -Target $t) { $builtOk++ } else { $builtFail++ }
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host (" " + (msg -Key "SummaryTitle")) -ForegroundColor Cyan
Write-Host (" " + (msg -Key "SummaryBuilt" -Arg0 $builtOk)) -ForegroundColor Cyan
Write-Host (" " + (msg -Key "SummaryFailed" -Arg0 $builtFail)) -ForegroundColor Cyan
Write-Host (" " + (msg -Key "SummarySkipped" -Arg0 $builtSkip)) -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# ---------------------------------------------------------------------------
# MSIX packaging (Windows GUI)
# ---------------------------------------------------------------------------
if ($BuildMsix) {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "[MSIX] Building MSIX package..." -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan

    $OutputDir = Join-Path $ProjectRoot "Apppackage"
    if (Test-Path $OutputDir) {
        Remove-Item "$OutputDir\*.msix" -Force -ErrorAction SilentlyContinue
    }

    try {
        winapp --version | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "winapp not found" }
    } catch {
        Write-Host "[ERROR] WinApp CLI is not installed. Install it via: winget install Microsoft.WinAppCLI" -ForegroundColor Red
        if (-not $Silent) { Read-Host (msg -Key "Exit") }
        exit 1
    }

    if (-not (Test-Path $ManifestPath)) {
        Write-Host "[ERROR] Manifest file not found at $ManifestPath" -ForegroundColor Red
        if (-not $Silent) { Read-Host (msg -Key "Exit") }
        exit 1
    }

    $CertPath = Join-Path $ProjectRoot "devcert.pfx"
    if (-not $SkipSign) {
        if (-not (Test-Path $CertPath)) {
            Write-Host "[MSIX] Certificate not found. Generating one from manifest..." -ForegroundColor Yellow
            winapp cert generate --manifest $ManifestPath --output $CertPath --install
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[ERROR] Failed to generate certificate." -ForegroundColor Red
                if (-not $Silent) { Read-Host (msg -Key "Exit") }
                exit 1
            }
        }
    }

    # Packaging source: build\bin\gui\Windows\<arch>
    $SourceDir = $null
    $guiWinRoot = Join-Path $ProjectRoot "build\bin\gui\Windows"
    foreach ($candidate in @($HostArch, "x64", "x86", "arm64") | Select-Object -Unique) {
        $dir = Join-Path $guiWinRoot $candidate
        if (Test-Path (Join-Path $dir "snishaper.exe")) { $SourceDir = $dir; break }
    }
    if (-not $SourceDir -and (Test-Path (Join-Path $ProjectRoot "build\bin\snishaper.exe"))) {
        # backward compatibility with the legacy flat layout
        $SourceDir = Join-Path $ProjectRoot "build\bin"
    }
    if (-not $SourceDir) {
        Write-Host "[ERROR] No Windows GUI output found (expected build\bin\gui\Windows\<arch>\snishaper.exe)" -ForegroundColor Red
        if (-not $Silent) { Read-Host (msg -Key "Exit") }
        exit 1
    }
    Write-Host "[MSIX] Source directory: $SourceDir" -ForegroundColor Green

    [xml]$ManifestXml = Get-Content $ManifestPath
    $PkgName = $ManifestXml.Package.Identity.Name
    $PkgVersion = $ManifestXml.Package.Identity.Version

    $ExePath = Join-Path $SourceDir "snishaper.exe"
    $fs = [System.IO.File]::OpenRead($ExePath)
    $fs.Seek(0x3C, [System.IO.SeekOrigin]::Begin) | Out-Null
    $peOffset = New-Object byte[] 4
    $fs.Read($peOffset, 0, 4) | Out-Null
    $offset = [BitConverter]::ToUInt32($peOffset, 0)
    $fs.Seek($offset + 4, [System.IO.SeekOrigin]::Begin) | Out-Null
    $machine = New-Object byte[] 2
    $fs.Read($machine, 0, 2) | Out-Null
    $fs.Close()
    $machineId = [BitConverter]::ToUInt16($machine, 0)
    switch ($machineId) {
        0x8664 { $PkgArch = "x64" }
        0xAA64 { $PkgArch = "arm64" }
        0x014C { $PkgArch = "x86" }
        default { $PkgArch = "unknown" }
    }
    Write-Host "[MSIX] Detected architecture: $PkgArch" -ForegroundColor Green

    $MsixFileName = "${PkgName}_${PkgVersion}_${PkgArch}.msix"

    winapp pack $SourceDir --manifest $ManifestPath --output (Join-Path $OutputDir $MsixFileName)
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[ERROR] winapp pack failed." -ForegroundColor Red
        if (-not $Silent) { Read-Host (msg -Key "Exit") }
        exit 1
    }

    $MsixFile = Get-Item (Join-Path $OutputDir $MsixFileName) -ErrorAction SilentlyContinue
    if (-not $MsixFile) {
        Write-Host "[ERROR] No .msix file found in $OutputDir." -ForegroundColor Red
        if (-not $Silent) { Read-Host (msg -Key "Exit") }
        exit 1
    }

    if (-not $SkipSign) {
        Write-Host "[MSIX] Signing the package..." -ForegroundColor Green
        winapp sign $MsixFile.FullName $CertPath
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[ERROR] winapp sign failed." -ForegroundColor Red
            if (-not $Silent) { Read-Host (msg -Key "Exit") }
            exit 1
        }
        Write-Host "[MSIX] Package signed successfully at $OutputDir" -ForegroundColor Green
    } else {
        Write-Host "[MSIX] Skipping signing as requested." -ForegroundColor Yellow
        $UnsignedName = "unsigned_" + $MsixFile.Name
        $NewPath = Join-Path $MsixFile.Directory $UnsignedName
        Rename-Item -Path $MsixFile.FullName -NewName $UnsignedName -ErrorAction Stop
        Write-Host "[MSIX] Unsigned package renamed to: $UnsignedName" -ForegroundColor Yellow
        Write-Host "[MSIX] Unsigned package located at $NewPath" -ForegroundColor Yellow
    }
    Write-Host ""
}

# ---------------------------------------------------------------------------
# Done
# ---------------------------------------------------------------------------
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host (msg -Key "AllDone") -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

if ($builtFail -ne 0) {
    Write-Host "[build] $builtFail target(s) failed." -ForegroundColor Red
    exit 1
}

if (-not $Silent) {
    Read-Host (msg -Key "Exit")
}
