param(
    [ValidateSet("frontend", "backend", "all")]
    [string]$Build,

    [ValidateSet("en", "cn", "ru")]
    [string]$Lang,

    [switch]$InstallDeps,

    [switch]$BuildMsix,

    [switch]$SkipSign,

    [switch]$Silent
)

$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "Requesting Administrator privileges..." -ForegroundColor Yellow
    $params = @()
    if ($Build)      { $params += "-Build";      $params += $Build }
    if ($Lang)       { $params += "-Lang";       $params += $Lang }
    if ($InstallDeps) { $params += "-InstallDeps" }
    if ($BuildMsix)  { $params += "-BuildMsix" }
    if ($SkipSign)   { $params += "-SkipSign" }
    if ($Silent)     { $params += "-Silent" }
    $paramStr = $params -join ' '
    Start-Process powershell.exe -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`" $paramStr"
    exit
} else {
    Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser -Force -ErrorAction SilentlyContinue
}

# Set console encoding to UTF-8 to properly display Chinese characters
try {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    $OutputEncoding = [System.Text.Encoding]::UTF8
    chcp 65001 | Out-Null
} catch {
    # If setting encoding fails, continue anyway
}

$ProjectRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ProjectRoot

# Kill any running snishaper instances before build
Get-Process -Name "snishaper" -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "[build] Killing snishaper process (PID: $($_.Id))..." -ForegroundColor Yellow
    $_ | Stop-Process -Force
}
Start-Sleep -Milliseconds 500

$messages = @{
    "LangTitle" = "Please select your language / 请选择语言 / Выберите язык"
    "LangOpt1" = "English"
    "LangOpt2" = "中文"
    "LangOpt3" = "Русский"
    "LangPrompt" = "Enter your choice (1, 2 or 3)"

    "EN_MenuTitle" = "       Project Build Menu"
    "EN_DepPrompt" = "Do you want to install frontend npm dependencies? (Y/N, default is N)"
    "EN_MsixPrompt" = "Do you want to build MSIX package? (Y/N, default N)"
    "EN_SignPrompt" = "Do you want to sign the MSIX package? (Y/N, default Y)"
    "EN_SelectTitle" = "Please select a build option:"
    "EN_Opt1" = "1. Build Frontend only"
    "EN_Opt2" = "2. Build Backend only"
    "EN_Opt3" = "3. Build Both Frontend and Backend"
    "EN_ChoicePrompt" = "Enter your choice (1, 2, or 3)"
    "EN_Start" = "Starting build process..."
    "EN_FrontEnter" = "[Frontend] Entering frontend directory..."
    "EN_FrontErrDir" = "[Frontend] ERROR: Failed to enter 'frontend' directory!"
    "EN_FrontInstall" = "[Frontend] Installing npm dependencies..."
    "EN_FrontErrInstall" = "[Frontend] ERROR: npm install failed!"
    "EN_FrontBuild" = "[Frontend] Running command: npm run build..."
    "EN_FrontErrBuild" = "[Frontend] ERROR: 'npm run build' failed!"
    "EN_FrontDone" = "[Frontend] Frontend build completed successfully!"
    "EN_BackStart" = "[Backend] Starting Go build..."
    "EN_BackInstallDeps" = "[Backend] Installing Go dependencies..."
    "EN_BackErrInstallDeps" = "[Backend] ERROR: go mod download failed!"
    "EN_BackErrBuild" = "[Backend] ERROR: Go build failed!"
    "EN_BackCopyCore" = "[Backend] Copying 'rules' folder..."
    "EN_BackCopyProxy" = "[Backend] Copying 'config' folder..."
    "EN_BackDone" = "[Backend] Backend build and file copy completed!"
    "EN_AllDone" = "All selected tasks finished successfully!"
    "EN_Exit" = "Press Enter to exit"

    "CN_MenuTitle" = "       项目构建菜单"
    "CN_DepPrompt" = "是否需要安装前端 npm 依赖？(Y/N，默认为 N)"
    "CN_MsixPrompt" = "是否需要构建 MSIX 安装包？(Y/N，默认为 N)"
    "CN_SignPrompt" = "是否对 MSIX 进行签名？(Y/N，默认为 Y)"
    "CN_SelectTitle" = "请选择构建选项："
    "CN_Opt1" = "1. 仅构建前端"
    "CN_Opt2" = "2. 仅构建后端"
    "CN_Opt3" = "3. 同时构建前后端"
    "CN_ChoicePrompt" = "请输入你的选择 (1, 2 或 3)"
    "CN_Start" = "开始执行构建流程..."
    "CN_FrontEnter" = "[前端] 正在进入 frontend 目录..."
    "CN_FrontErrDir" = "[前端] 错误：无法进入 'frontend' 目录！"
    "CN_FrontInstall" = "[前端] 正在安装 npm 依赖..."
    "CN_FrontErrInstall" = "[前端] 错误：npm install 安装失败！"
    "CN_FrontBuild" = "[前端] 正在执行命令：npm run build..."
    "CN_FrontErrBuild" = "[前端] 错误：'npm run build' 构建失败！"
    "CN_FrontDone" = "[前端] 前端构建成功完成！"
    "CN_BackStart" = "[后端] 正在开始 Go 编译..."
    "CN_BackInstallDeps" = "[后端] 正在安装 Go 依赖..."
    "CN_BackErrInstallDeps" = "[后端] 错误：go mod download 失败！"
    "CN_BackErrBuild" = "[后端] 错误：Go 编译失败！"
    "CN_BackCopyCore" = "[后端] 正在复制 'rules' 文件夹..."
    "CN_BackCopyProxy" = "[后端] 正在复制 'config' 文件夹..."
    "CN_BackDone" = "[后端] 后端编译与文件复制完成！"
    "CN_AllDone" = "所有选定的任务已成功完成！"
    "CN_Exit" = "按回车键退出"

    "RU_MenuTitle" = "       Меню сборки проекта"
    "RU_DepPrompt" = "Установить npm зависимости фронтенда? (Y/N, по умолчанию N)"
    "RU_MsixPrompt" = "Создать MSIX-пакет? (Y/N, по умолчанию N)"
    "RU_SignPrompt" = "Подписать MSIX-пакет? (Y/N, по умолчанию Y)"
    "RU_SelectTitle" = "Выберите вариант сборки:"
    "RU_Opt1" = "1. Собрать только фронтенд"
    "RU_Opt2" = "2. Собрать только бэкенд"
    "RU_Opt3" = "3. Собрать фронтенд и бэкенд"
    "RU_ChoicePrompt" = "Введите ваш выбор (1, 2 или 3)"
    "RU_Start" = "Начало сборки..."
    "RU_FrontEnter" = "[Фронтенд] Переход в директорию frontend..."
    "RU_FrontErrDir" = "[Фронтенд] ОШИБКА: Не удалось войти в директорию 'frontend'!"
    "RU_FrontInstall" = "[Фронтенд] Установка npm зависимостей..."
    "RU_FrontErrInstall" = "[Фронтенд] ОШИБКА: npm install не удался!"
    "RU_FrontBuild" = "[Фронтенд] Запуск команды: npm run build..."
    "RU_FrontErrBuild" = "[Фронтенд] ОШИБКА: 'npm run build' не удался!"
    "RU_FrontDone" = "[Фронтенд] Сборка фронтенда завершена успешно!"
    "RU_BackStart" = "[Бэкенд] Начало сборки Go..."
    "RU_BackInstallDeps" = "[Бэкенд] Установка Go зависимостей..."
    "RU_BackErrInstallDeps" = "[Бэкенд] ОШИБКА: go mod download не удался!"
    "RU_BackErrBuild" = "[Бэкенд] ОШИБКА: Сборка Go не удалась!"
    "RU_BackCopyCore" = "[Бэкенд] Копирование папки 'rules'..."
    "RU_BackCopyProxy" = "[Бэкенд] Копирование папки 'config'..."
    "RU_BackDone" = "[Бэкенд] Сборка бэкенда и копирование файлов завершены!"
    "RU_AllDone" = "Все выбранные задачи успешно завершены!"
    "RU_Exit" = "Нажмите Enter для выхода"
}

# --- Silent mode defaults ---
if ($Silent) {
    if (-not $Build) { $Build = "all" }
    if (-not $Lang) { $Lang = "EN" }
}

# --- Resolve language ---
if ($Lang) {
    $lang = $Lang.ToUpper()
} elseif (-not $Silent) {
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host $messages["LangTitle"] -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "1. $($messages['LangOpt1'])"
    Write-Host "2. $($messages['LangOpt2'])"
    Write-Host "3. $($messages['LangOpt3'])"
    Write-Host ""
    $langChoice = Read-Host $messages["LangPrompt"]

    if ($langChoice -eq "2") {
        $lang = "CN"
    } elseif ($langChoice -eq "1") {
        $lang = "EN"
    } elseif ($langChoice -eq "3") {
        $lang = "RU"
    } else {
        Write-Host "Invalid choice, defaulting to English..." -ForegroundColor Yellow
        $lang = "EN"
    }
} else {
    $lang = "EN"
}

# --- Resolve build target and MSIX/sign options ---
$installDepsInput = $null
$msixInput = $null
$signInput = $null

if ($Build) {
    switch ($Build) {
        "frontend" { $choice = "1" }
        "backend"  { $choice = "2" }
        "all"      { $choice = "3" }
    }
} elseif (-not $Silent) {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host $messages["$($lang)_MenuTitle"] -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host ""

    if (-not $PSBoundParameters.ContainsKey('InstallDeps')) {
        $installDepsInput = Read-Host $messages["$($lang)_DepPrompt"]
    }

    if (-not $PSBoundParameters.ContainsKey('BuildMsix')) {
        $msixInput = Read-Host $messages["$($lang)_MsixPrompt"]
    }

    if ($msixInput -eq "Y" -or $msixInput -eq "y" -or $BuildMsix) {
        if (-not $PSBoundParameters.ContainsKey('SkipSign')) {
            $signInput = Read-Host $messages["$($lang)_SignPrompt"]
        }
    }

    Write-Host ""
    Write-Host $messages["$($lang)_SelectTitle"] -ForegroundColor Yellow
    Write-Host $messages["$($lang)_Opt1"]
    Write-Host $messages["$($lang)_Opt2"]
    Write-Host $messages["$($lang)_Opt3"]
    Write-Host ""

    $choice = Read-Host $messages["$($lang)_ChoicePrompt"]
} else {
    $choice = "3"
}

# Validate choice
if ($choice -ne "1" -and $choice -ne "2" -and $choice -ne "3") {
    Write-Host "[ERROR] Invalid choice: $choice. Please enter 1, 2, or 3." -ForegroundColor Red
    if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
    exit 1
}

# --- Resolve InstallDeps when interactive ---
if (-not $Build -and -not $Silent -and -not $PSBoundParameters.ContainsKey('InstallDeps')) {
    if ([string]::IsNullOrWhiteSpace($installDepsInput)) {
        $installDepsInput = "N"
    }
    if ($installDepsInput -eq "Y" -or $installDepsInput -eq "y") {
        $InstallDeps = $true
    }
}

# --- Resolve BuildMsix when interactive ---
if (-not $Build -and -not $Silent -and -not $PSBoundParameters.ContainsKey('BuildMsix')) {
    if ([string]::IsNullOrWhiteSpace($msixInput)) {
        $msixInput = "N"
    }
    if ($msixInput -eq "Y" -or $msixInput -eq "y") {
        $BuildMsix = $true
    }
}

# --- Resolve SkipSign when interactive (if BuildMsix is true) ---
if ($BuildMsix -and -not $Silent -and -not $PSBoundParameters.ContainsKey('SkipSign')) {
    if ([string]::IsNullOrWhiteSpace($signInput)) {
        $signInput = "Y"
    }
    if ($signInput -eq "N" -or $signInput -eq "n") {
        $SkipSign = $true
    } else {
        $SkipSign = $false
    }
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host $messages["$($lang)_Start"] -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

# ---------- Frontend Build ----------
if ($choice -eq "1" -or $choice -eq "3") {
    Write-Host ""
    Write-Host $messages["$($lang)_FrontEnter"] -ForegroundColor Green
    
    try {
        npm --version | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "npm not found"
        }
    } catch {
        Write-Host "[ERROR] npm is not installed or not in PATH. Please install Node.js first." -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }
    
    $FrontendPath = Join-Path $ProjectRoot "frontend"
    if (-not (Test-Path $FrontendPath -PathType Container)) {
        Write-Host "[ERROR] Cannot find frontend directory: $FrontendPath" -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    try {
        Set-Location $FrontendPath
    } catch {
        Write-Host $messages["$($lang)_FrontErrDir"] -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    if ($InstallDeps) {
        Write-Host $messages["$($lang)_FrontInstall"] -ForegroundColor Green
        npm install
        if ($LASTEXITCODE -ne 0) {
            Write-Host $messages["$($lang)_FrontErrInstall"] -ForegroundColor Red
            Set-Location $ProjectRoot
            if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
            exit 1
        }
    }

    Write-Host $messages["$($lang)_FrontBuild"] -ForegroundColor Green
    npm run build
    if ($LASTEXITCODE -ne 0) {
        Write-Host $messages["$($lang)_FrontErrBuild"] -ForegroundColor Red
        Set-Location $ProjectRoot
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    Write-Host $messages["$($lang)_FrontDone"] -ForegroundColor Green
    Set-Location $ProjectRoot
    Write-Host ""
}

# ---------- Backend Build ----------
if ($choice -eq "2" -or $choice -eq "3") {
    try {
        go version | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "Go not found"
        }
    } catch {
        Write-Host "[ERROR] Go is not installed or not in PATH. Please install Go first." -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }
    
    Write-Host $messages["$($lang)_BackStart"] -ForegroundColor Green
    
    Write-Host $messages["$($lang)_BackInstallDeps"] -ForegroundColor Green
    go mod download
    if ($LASTEXITCODE -ne 0) {
        Write-Host $messages["$($lang)_BackErrInstallDeps"] -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }
    
    $BuildDir = Join-Path $ProjectRoot "build"
    $BuildBinPath = Join-Path $BuildDir "bin"
    if (-not (Test-Path $BuildBinPath -PathType Container)) {
        Write-Host "[Backend] Creating build/bin directory..." -ForegroundColor Green
        New-Item -ItemType Directory -Path $BuildBinPath -Force | Out-Null
    }
    
    go build -ldflags="-s -w -H windowsgui" -o "$BuildBinPath\snishaper.exe" .
    if ($LASTEXITCODE -ne 0) {
        Write-Host $messages["$($lang)_BackErrBuild"] -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    # 复制 rules 文件夹到 build/bin
    Write-Host $messages["$($lang)_BackCopyCore"] -ForegroundColor Green
    $RulesSrc = Join-Path $ProjectRoot "rules"
    $RulesDst = Join-Path $BuildBinPath "rules"
    if (Test-Path $RulesSrc -PathType Container) {
        try {
            Copy-Item -Path $RulesSrc -Destination $RulesDst -Recurse -Force -ErrorAction Stop
        } catch {
            Write-Host "[ERROR] Failed to copy 'rules' folder! $_" -ForegroundColor Red
            if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
            exit 1
        }
    } else {
        Write-Host "[WARNING] 'rules' folder not found, skipping copy." -ForegroundColor Yellow
    }
    
    # 复制 config 文件夹到 build/bin
    Write-Host $messages["$($lang)_BackCopyProxy"] -ForegroundColor Green
    $ConfigSrc = Join-Path $ProjectRoot "config"
    $ConfigDst = Join-Path $BuildBinPath "config"
    if (Test-Path $ConfigSrc -PathType Container) {
        try {
            Copy-Item -Path $ConfigSrc -Destination $ConfigDst -Recurse -Force -ErrorAction Stop
        } catch {
            Write-Host "[ERROR] Failed to copy 'config' folder! $_" -ForegroundColor Red
            if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
            exit 1
        }
    } else {
        Write-Host "[WARNING] 'config' folder not found, skipping copy." -ForegroundColor Yellow
    }

    Write-Host $messages["$($lang)_BackDone"] -ForegroundColor Green
    Write-Host ""
}

# ---------- MSIX Package Build (optional) ----------
if ($BuildMsix) {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Cyan
    Write-Host "[MSIX] Building MSIX package..." -ForegroundColor Cyan
    Write-Host "==========================================" -ForegroundColor Cyan

    # 0. Clean previous packages
    $OutputDir = Join-Path $ProjectRoot "Apppackage"
    if (Test-Path $OutputDir) {
        Remove-Item "$OutputDir\*.msix" -Force -ErrorAction SilentlyContinue
    }

    # 1. Check if winapp CLI is installed
    try {
        winapp --version | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "winapp not found"
        }
    } catch {
        Write-Host "[ERROR] WinApp CLI is not installed. Please install it via: winget install Microsoft.WinAppCLI" -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    # 2. Check manifest
    $ManifestPath = Join-Path $ProjectRoot "Package.appxmanifest"
    if (-not (Test-Path $ManifestPath)) {
        Write-Host "[ERROR] Manifest file not found at $ManifestPath" -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    # 3. Ensure certificate exists, generate if not (only if signing is not skipped)
    $CertPath = Join-Path $ProjectRoot "devcert.pfx"
    if (-not $SkipSign) {
        if (-not (Test-Path $CertPath)) {
            Write-Host "[MSIX] Certificate not found. Generating one from manifest..." -ForegroundColor Yellow
            winapp cert generate --manifest $ManifestPath --output $CertPath --install
            if ($LASTEXITCODE -ne 0) {
                Write-Host "[ERROR] Failed to generate certificate." -ForegroundColor Red
                if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
                exit 1
            }
        }
    }

    # 4. Pack - only pack build\bin directory
    Write-Host "[MSIX] Running winapp pack from build\bin..." -ForegroundColor Green
    $SourceDir = Join-Path $ProjectRoot "build\bin"
    
    if (-not (Test-Path $SourceDir -PathType Container)) {
        Write-Host "[ERROR] Source directory not found: $SourceDir" -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

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
        0x8664 { $Arch = "x64" }
        0xAA64 { $Arch = "arm64" }
        0x014C { $Arch = "x86" }
        default { $Arch = "unknown" }
    }
    Write-Host "[MSIX] Detected architecture: $Arch" -ForegroundColor Green

    $MsixFileName = "${PkgName}_${PkgVersion}_${Arch}.msix"
    
    winapp pack $SourceDir --manifest $ManifestPath --output (Join-Path $OutputDir $MsixFileName)
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[ERROR] winapp pack failed." -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    # 5. Find the generated .msix file
    $MsixFile = Get-Item (Join-Path $OutputDir $MsixFileName) -ErrorAction SilentlyContinue
    if (-not $MsixFile) {
        Write-Host "[ERROR] No .msix file found in $OutputDir." -ForegroundColor Red
        if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
        exit 1
    }

    # 6. Sign or rename with unsigned_ prefix
    if (-not $SkipSign) {
        Write-Host "[MSIX] Signing the package..." -ForegroundColor Green
        winapp sign $MsixFile.FullName $CertPath
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[ERROR] winapp sign failed." -ForegroundColor Red
            if (-not $Silent) { Read-Host $messages["$($lang)_Exit"] }
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

# ---------- Done ----------
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host $messages["$($lang)_AllDone"] -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

if (-not $Silent) {
    Read-Host $messages["$($lang)_Exit"]
}
