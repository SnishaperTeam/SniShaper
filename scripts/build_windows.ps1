$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
$isAdmin = $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)

if (-not $isAdmin) {
    Write-Host "Requesting Administrator privileges..." -ForegroundColor Yellow
    Start-Process powershell.exe -Verb RunAs -ArgumentList "-NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`""
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

$messages = @{
    "LangTitle" = "Please select your language / 请选择语言"
    "LangOpt1" = "English"
    "LangOpt2" = "中文"
    "LangPrompt" = "Enter your choice (1 or 2)"

    "EN_MenuTitle" = "       Project Build Menu"
    "EN_DepPrompt" = "Do you want to install frontend npm dependencies? (Y/N, default is N)"
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
    "EN_BackErrBuild" = "[Backend] ERROR: Go build failed!"
    "EN_BackCopyCore" = "[Backend] Copying 'core' folder..."
    "EN_BackCopyProxy" = "[Backend] Copying 'proxy' folder..."
    "EN_BackCopyRuntime" = "[Backend] Copying 'runtime' folder..."
    "EN_BackDone" = "[Backend] Backend build and file copy completed!"
    "EN_AllDone" = "All selected tasks finished successfully!"
    "EN_Exit" = "Press Enter to exit"

    "CN_MenuTitle" = "       项目构建菜单"
    "CN_DepPrompt" = "是否需要安装前端 npm 依赖？(Y/N，默认为 N)"
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
    "CN_BackErrBuild" = "[后端] 错误：Go 编译失败！"
    "CN_BackCopyCore" = "[后端] 正在复制 'core' 文件夹..."
    "CN_BackCopyProxy" = "[后端] 正在复制 'proxy' 文件夹..."
    "CN_BackCopyRuntime" = "[后端] 正在复制 'runtime' 文件夹..."
    "CN_BackDone" = "[后端] 后端编译与文件复制完成！"
    "CN_AllDone" = "所有选定的任务已成功完成！"
    "CN_Exit" = "按回车键退出"
}

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host $messages["LangTitle"] -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "1. $($messages['LangOpt1'])"
Write-Host "2. $($messages['LangOpt2'])"
Write-Host ""
$langChoice = Read-Host $messages["LangPrompt"]

if ($langChoice -eq "2") {
    $lang = "CN"
} elseif ($langChoice -eq "1") {
    $lang = "EN"
} else {
    Write-Host "Invalid choice, defaulting to English..." -ForegroundColor Yellow
    $lang = "EN"
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host $messages["$($lang)_MenuTitle"] -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

$installDeps = Read-Host $messages["$($lang)_DepPrompt"]

Write-Host ""
Write-Host $messages["$($lang)_SelectTitle"] -ForegroundColor Yellow
Write-Host $messages["$($lang)_Opt1"]
Write-Host $messages["$($lang)_Opt2"]
Write-Host $messages["$($lang)_Opt3"]
Write-Host ""

$choice = Read-Host $messages["$($lang)_ChoicePrompt"]

# Validate choice
if ($choice -ne "1" -and $choice -ne "2" -and $choice -ne "3") {
    Write-Host "[ERROR] Invalid choice: $choice. Please enter 1, 2, or 3." -ForegroundColor Red
    Read-Host $messages["$($lang)_Exit"]
    exit 1
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host $messages["$($lang)_Start"] -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan

if ($choice -eq "1" -or $choice -eq "3") {
    Write-Host ""
    Write-Host $messages["$($lang)_FrontEnter"] -ForegroundColor Green
    
    # Check if npm is available
    try {
        npm --version | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "npm not found"
        }
    } catch {
        Write-Host "[ERROR] npm is not installed or not in PATH. Please install Node.js first." -ForegroundColor Red
        Read-Host $messages["$($lang)_Exit"]
        exit 1
    }
    
    $FrontendPath = Join-Path $ProjectRoot "frontend"
    if (-not (Test-Path $FrontendPath -PathType Container)) {
        Write-Host "[ERROR] Cannot find frontend directory: $FrontendPath" -ForegroundColor Red
        Read-Host $messages["$($lang)_Exit"]
        exit 1
    }

    try {
        Set-Location $FrontendPath
    } catch {
        Write-Host $messages["$($lang)_FrontErrDir"] -ForegroundColor Red
        Read-Host $messages["$($lang)_Exit"]
        exit 1
    }

    # Handle empty input as default (N)
    if ([string]::IsNullOrWhiteSpace($installDeps)) {
        $installDeps = "N"
    }
    
    if ($installDeps -eq "Y" -or $installDeps -eq "y") {
        Write-Host $messages["$($lang)_FrontInstall"] -ForegroundColor Green
        npm install
        if ($LASTEXITCODE -ne 0) {
            Write-Host $messages["$($lang)_FrontErrInstall"] -ForegroundColor Red
            Set-Location $ProjectRoot
            Read-Host $messages["$($lang)_Exit"]
            exit 1
        }
    }

    Write-Host $messages["$($lang)_FrontBuild"] -ForegroundColor Green
    npm run build
    if ($LASTEXITCODE -ne 0) {
        Write-Host $messages["$($lang)_FrontErrBuild"] -ForegroundColor Red
        Set-Location $ProjectRoot
        Read-Host $messages["$($lang)_Exit"]
        exit 1
    }

    Write-Host $messages["$($lang)_FrontDone"] -ForegroundColor Green
    Set-Location $ProjectRoot
    Write-Host ""
}

if ($choice -eq "2" -or $choice -eq "3") {
    # Check if Go is available
    try {
        go version | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "Go not found"
        }
    } catch {
        Write-Host "[ERROR] Go is not installed or not in PATH. Please install Go first." -ForegroundColor Red
        Read-Host $messages["$($lang)_Exit"]
        exit 1
    }
    
    Write-Host $messages["$($lang)_BackStart"] -ForegroundColor Green
    
    # Ensure build/bin directory exists
    $BuildDir = Join-Path $ProjectRoot "build"
    $BuildBinPath = Join-Path $BuildDir "bin"
    if (-not (Test-Path $BuildBinPath -PathType Container)) {
        Write-Host "[Backend] Creating build/bin directory..." -ForegroundColor Green
        New-Item -ItemType Directory -Path $BuildBinPath -Force | Out-Null
    }
    
    go build -ldflags="-s -w" -o "$BuildBinPath\snishaper.exe" .
    if ($LASTEXITCODE -ne 0) {
        Write-Host $messages["$($lang)_BackErrBuild"] -ForegroundColor Red
        Read-Host $messages["$($lang)_Exit"]
        exit 1
    }

    Write-Host $messages["$($lang)_BackCopyCore"] -ForegroundColor Green
    $CoreSrc = Join-Path $ProjectRoot "core"
    $CoreDst = Join-Path $BuildBinPath "core"
    if (Test-Path $CoreSrc -PathType Container) {
        try {
            Copy-Item -Path $CoreSrc -Destination $CoreDst -Recurse -Force -ErrorAction Stop
        } catch {
            Write-Host "[ERROR] Failed to copy 'core' folder! $_" -ForegroundColor Red
            Read-Host $messages["$($lang)_Exit"]
            exit 1
        }
    } else {
        Write-Host "[WARNING] 'core' folder not found, skipping copy." -ForegroundColor Yellow
    }
    
    Write-Host $messages["$($lang)_BackCopyProxy"] -ForegroundColor Green
    $ProxySrc = Join-Path $ProjectRoot "proxy"
    $ProxyDst = Join-Path $BuildBinPath "proxy"
    if (Test-Path $ProxySrc -PathType Container) {
        try {
            Copy-Item -Path $ProxySrc -Destination $ProxyDst -Recurse -Force -ErrorAction Stop
        } catch {
            Write-Host "[ERROR] Failed to copy 'proxy' folder! $_" -ForegroundColor Red
            Read-Host $messages["$($lang)_Exit"]
            exit 1
        }
    } else {
        Write-Host "[WARNING] 'proxy' folder not found, skipping copy." -ForegroundColor Yellow
    }
    
    Write-Host $messages["$($lang)_BackCopyRuntime"] -ForegroundColor Green
    $RuntimeSrc = Join-Path $ProjectRoot "runtime"
    $RuntimeDst = Join-Path $BuildBinPath "runtime"
    if (Test-Path $RuntimeSrc -PathType Container) {
        try {
            Copy-Item -Path $RuntimeSrc -Destination $RuntimeDst -Recurse -Force -ErrorAction Stop
        } catch {
            Write-Host "[ERROR] Failed to copy 'runtime' folder! $_" -ForegroundColor Red
            Read-Host $messages["$($lang)_Exit"]
            exit 1
        }
    } else {
        Write-Host "[WARNING] 'runtime' folder not found, skipping copy." -ForegroundColor Yellow
    }

    Write-Host $messages["$($lang)_BackDone"] -ForegroundColor Green
    Write-Host ""
}

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host $messages["$($lang)_AllDone"] -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Read-Host $messages["$($lang)_Exit"]