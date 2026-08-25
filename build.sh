#!/usr/bin/env bash
#
# build.sh — SniShaper unified build script (Unix / Linux / macOS)
#
# Feature-equivalent with build_windows.ps1 — same parameters, same
# interactive flow, same i18n (EN/CN/RU), same default-to-full-build.
#
# Usage:
#   ./build.sh                              # Interactive mode (menu)
#   ./build.sh --build linux,gui,all         # Linux GUI (frontend + backend)
#   ./build.sh --build all,all,all           # Full build (GUI + CLI, all platforms)
#   ./build.sh --build all,cli,all           # CLI only (cross-platform, all archs)
#   ./build.sh --build linux,gui,frontend    # Frontend only
#   ./build.sh --build linux,gui,backend     # Backend only
#   ./build.sh --lang en                     # Set language (en|cn|ru)
#   ./build.sh --arch x64                    # x64 / arm64 / x86 / all (default=all)
#   ./build.sh --install-deps               # Install npm deps before build
#   ./build.sh --gtk3                       # Use GTK3 instead of GTK4
#   ./build.sh --silent                     # No prompts, full build default
#   ./build.sh --help                       # Show this help
#
# Legacy shortcuts (still work, map to --build):
#   ./build.sh --with-frontend              # = --build linux,gui,all --install-deps
#   ./build.sh --gui                        # = --build linux,gui,all
#   ./build.sh --cli                        # = --build all,cli,all
#   ./build.sh --all                        # = --build all,all,all
#
# Default (no --build): full build (All,All,All = GUI+CLI, all platforms, all archs)
#
# Output:
#   GUI: build/bin/SniShaper (with rules/ config/ seed files)
#   CLI: build/bin/cli/snishaper-cli-<os>-<arch>[.exe]
#
set -euo pipefail

cd "$(dirname "$0")"

# ---------- Go module proxy (same as before, overridable via env) ----------
if [ -z "${GOPROXY:-}" ]; then
    export GOPROXY="https://goproxy.cn,direct"
fi

# ---------- i18n messages (EN/CN/RU) — identical keys to build_windows.ps1 ----------
declare -A MSG
MSG=(
    ["LangTitle"]="Please select your language / 请选择语言 / Выберите язык"
    ["LangOpt1"]="English"
    ["LangOpt2"]="中文"
    ["LangOpt3"]="Русский"
    ["LangPrompt"]="Enter your choice (1, 2 or 3)"

    ["EN_MenuTitle"]="       Project Build Menu"
    ["EN_DepPrompt"]="Do you want to install frontend npm dependencies? (Y/N, default is N)"
    ["EN_SelectTitle"]="Please select a build option:"
    ["EN_Opt1"]="1. Linux GUI (frontend + backend)"
    ["EN_Opt2"]="2. CLI only (headless, cross-platform)"
    ["EN_Opt3"]="3. Full build (GUI + CLI, all platforms)"
    ["EN_Opt4"]="4. Full cross-platform CLI (all architectures)"
    ["EN_ChoicePrompt"]="Enter your choice (1-4, default=3)"
    ["EN_Start"]="Starting build process..."
    ["EN_FrontEnter"]="[Frontend] Entering frontend directory..."
    ["EN_FrontErrDir"]="[Frontend] ERROR: Failed to enter 'frontend' directory!"
    ["EN_FrontInstall"]="[Frontend] Installing npm dependencies..."
    ["EN_FrontErrInstall"]="[Frontend] ERROR: npm install failed!"
    ["EN_FrontBuild"]="[Frontend] Running command: npm run build..."
    ["EN_FrontErrBuild"]="[Frontend] ERROR: 'npm run build' failed!"
    ["EN_FrontDone"]="[Frontend] Frontend build completed successfully!"
    ["EN_BackStart"]="[Backend] Starting Go build..."
    ["EN_BackInstallDeps"]="[Backend] Installing Go dependencies..."
    ["EN_BackErrInstallDeps"]="[Backend] ERROR: go mod download failed!"
    ["EN_BackErrBuild"]="[Backend] ERROR: Go build failed!"
    ["EN_BackCopyCore"]="[Backend] Copying 'rules' folder..."
    ["EN_BackCopyProxy"]="[Backend] Copying 'config' folder..."
    ["EN_BackDone"]="[Backend] Backend build and file copy completed!"
    ["EN_AllDone"]="All selected tasks finished successfully!"
    ["EN_Exit"]="Press Enter to exit"
    ["EN_BackBuildVersion"]="[Backend] Build version: {0}"
    ["EN_CliPrompt"]="Build the headless CLI as well? (Y/N, default N)"
    ["EN_ArchPrompt"]="Select target architecture (1=x64, 2=arm64, 3=x86 Windows-only, default=1)"

    ["CN_MenuTitle"]="       项目构建菜单"
    ["CN_DepPrompt"]="是否需要安装前端 npm 依赖？(Y/N，默认为 N)"
    ["CN_SelectTitle"]="请选择构建选项："
    ["CN_Opt1"]="1. Linux GUI（前端 + 后端）"
    ["CN_Opt2"]="2. 仅 CLI（headless，跨平台）"
    ["CN_Opt3"]="3. 全量构建（GUI + CLI，全平台）"
    ["CN_Opt4"]="4. 全架构 CLI（跨平台，amd64+arm64）"
    ["CN_ChoicePrompt"]="请输入你的选择 (1-4，默认3)"
    ["CN_Start"]="开始执行构建流程..."
    ["CN_FrontEnter"]="[前端] 正在进入 frontend 目录..."
    ["CN_FrontErrDir"]="[前端] 错误：无法进入 'frontend' 目录！"
    ["CN_FrontInstall"]="[前端] 正在安装 npm 依赖..."
    ["CN_FrontErrInstall"]="[前端] 错误：npm install 安装失败！"
    ["CN_FrontBuild"]="[前端] 正在执行命令：npm run build..."
    ["CN_FrontErrBuild"]="[前端] 错误：'npm run build' 构建失败！"
    ["CN_FrontDone"]="[前端] 前端构建成功完成！"
    ["CN_BackStart"]="[后端] 正在开始 Go 编译..."
    ["CN_BackInstallDeps"]="[后端] 正在安装 Go 依赖..."
    ["CN_BackErrInstallDeps"]="[后端] 错误：go mod download 失败！"
    ["CN_BackErrBuild"]="[后端] 错误：Go 编译失败！"
    ["CN_BackCopyCore"]="[后端] 正在复制 'rules' 文件夹..."
    ["CN_BackCopyProxy"]="[后端] 正在复制 'config' 文件夹..."
    ["CN_BackDone"]="[后端] 后端编译与文件复制完成！"
    ["CN_AllDone"]="所有选定的任务已成功完成！"
    ["CN_Exit"]="按回车键退出"
    ["CN_BackBuildVersion"]="[后端] 构建版本：{0}"
    ["CN_CliPrompt"]="是否同时构建 CLI（headless 跨平台）？(Y/N，默认为 N)"
    ["CN_ArchPrompt"]="请选择目标架构 (1=x64（默认），2=arm64，3=x86 仅 Windows)"

    ["RU_MenuTitle"]="       Меню сборки проекта"
    ["RU_DepPrompt"]="Установить npm зависимости фронтенда? (Y/N, по умолчанию N)"
    ["RU_SelectTitle"]="Выберите вариант сборки:"
    ["RU_Opt1"]="1. Linux GUI (фронтенд + бэкенд)"
    ["RU_Opt2"]="2. Только CLI (headless, кроссплатформенный)"
    ["RU_Opt3"]="3. Полная сборка (GUI + CLI, все платформы)"
    ["RU_Opt4"]="4. CLI для всех архитектур (amd64+arm64)"
    ["RU_ChoicePrompt"]="Введите ваш выбор (1-4, по умолчанию 3)"
    ["RU_Start"]="Начало сборки..."
    ["RU_FrontEnter"]="[Фронтенд] Переход в директорию frontend..."
    ["RU_FrontErrDir"]="[Фронтенд] ОШИБКА: Не удалось войти в директорию 'frontend'!"
    ["RU_FrontInstall"]="[Фронтенд] Установка npm зависимостей..."
    ["RU_FrontErrInstall"]="[Фронтенд] ОШИБКА: npm install не удался!"
    ["RU_FrontBuild"]="[Фронтенд] Запуск команды: npm run build..."
    ["RU_FrontErrBuild"]="[Фронтенд] ОШИБКА: 'npm run build' не удался!"
    ["RU_FrontDone"]="[Фронтенд] Сборка фронтенда завершена успешно!"
    ["RU_BackStart"]="[Бэкенд] Начало сборки Go..."
    ["RU_BackInstallDeps"]="[Бэкенд] Установка Go зависимостей..."
    ["RU_BackErrInstallDeps"]="[Бэкенд] ОШИБКА: go mod download не удался!"
    ["RU_BackErrBuild"]="[Бэкенд] ОШИБКА: Сборка Go не удалась!"
    ["RU_BackCopyCore"]="[Бэкенд] Копирование папки 'rules'..."
    ["RU_BackCopyProxy"]="[Бэкенд] Копирование папки 'config'..."
    ["RU_BackDone"]="[Бэкенд] Сборка бэкенда и копирование файлов завершены!"
    ["RU_AllDone"]="Все выбранные задачи успешно завершены!"
    ["RU_Exit"]="Нажмите Enter для выхода"
    ["RU_BackBuildVersion"]="[Бэкенд] Версия сборки: {0}"
    ["RU_CliPrompt"]="Также собрать headless CLI? (Y/N, по умолчанию N)"
    ["RU_ArchPrompt"]="Выберите целевую архитектуру (1=x64 (по умолчанию), 2=arm64, 3=x86 только Windows)"
)

# Helper: get message for current language
# Usage: m "KEY" [params...]
m() {
    local key="$1"
    shift
    local val="${MSG["${LANG_CODE}_${key}"]:-${MSG["$key"]:-$key}}"
    # Simple {0} placeholder replacement
    if [ "$#" -gt 0 ]; then
        val="${val//\{0\}/$1}"
    fi
    printf '%s' "$val"
}

# ---------- Default parameters (match ps1: All,All,All when missing) ----------
BUILD_SYSTEM="All"
BUILD_MODE="All"
BUILD_SCOPE="All"
LANG_CODE=""
ARCH_PARAM=""
INSTALL_DEPS=0
GTK3=0
SILENT=0
CLI_FLAG=0
ALL_FLAG=0
GUI_FLAG=0
WITH_FRONTEND=0
BUILD_PARAM_SET=0

# ---------- Helper: capitalize first letter ----------
cap_first() {
    local s
    s="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')"
    local first
    first="$(printf '%s' "${s:0:1}" | tr '[:lower:]' '[:upper:]')"
    printf '%s%s' "$first" "${s:1}"
}

# ---------- Parameter parsing ----------
ARGS=("$@")
_i=0
while [ "$_i" -lt "${#ARGS[@]}" ]; do
    case "${ARGS[$_i]}" in
        --build)
            _i=$((_i+1))
            if [ "$_i" -ge "${#ARGS[@]}" ]; then
                echo "[build] --build requires a value (e.g. --build all,all,all)" >&2
                exit 1
            fi
            # Parse comma-separated: system,mode,scope
            IFS=',' read -r _sys _mode _scope <<<"${ARGS[$_i]}"
            [ -n "${_sys:-}" ]   && BUILD_SYSTEM="$(cap_first "$_sys")"
            [ -n "${_mode:-}" ]  && BUILD_MODE="$(cap_first "$_mode")"
            [ -n "${_scope:-}" ] && BUILD_SCOPE="$(cap_first "$_scope")"
            BUILD_PARAM_SET=1
            ;;
        --lang)
            _i=$((_i+1))
            if [ "$_i" -ge "${#ARGS[@]}" ]; then
                echo "[build] --lang requires a value (en|cn|ru)" >&2
                exit 1
            fi
            LANG_CODE="$(printf '%s' "${ARGS[$_i]}" | tr '[:lower:]' '[:upper:]')"
            case "$LANG_CODE" in
                EN|CN|RU) ;;
                *) echo "[build] --lang must be en, cn, or ru" >&2; exit 1 ;;
            esac
            ;;
        --arch)
            _i=$((_i+1))
            if [ "$_i" -ge "${#ARGS[@]}" ]; then
                echo "[build] --arch requires a value (x64|arm64|x86|all)" >&2
                exit 1
            fi
            ARCH_PARAM="${ARGS[$_i]}"
            ;;
        --install-deps) INSTALL_DEPS=1 ;;
        --gtk3)          GTK3=1 ;;
        --silent)        SILENT=1 ;;
        --cli)           CLI_FLAG=1; BUILD_PARAM_SET=1 ;;
        --all)           ALL_FLAG=1; BUILD_PARAM_SET=1 ;;
        --gui)           GUI_FLAG=1; BUILD_PARAM_SET=1 ;;
        --with-frontend) WITH_FRONTEND=1; INSTALL_DEPS=1; BUILD_PARAM_SET=1 ;;
        --help|-h)
            grep '^#' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) echo "[build] unknown parameter: ${ARGS[$_i]} (--help for usage)" >&2; exit 1 ;;
    esac
    _i=$((_i+1))
done

# ---------- Resolve legacy shortcuts → --build equivalents ----------
if [ "$WITH_FRONTEND" = "1" ]; then
    BUILD_SYSTEM="Linux"; BUILD_MODE="Gui"; BUILD_SCOPE="All"
fi
if [ "$GUI_FLAG" = "1" ]; then
    BUILD_SYSTEM="Linux"; BUILD_MODE="Gui"; BUILD_SCOPE="All"
fi
if [ "$CLI_FLAG" = "1" ] && [ "$ALL_FLAG" = "0" ]; then
    BUILD_SYSTEM="All"; BUILD_MODE="Cli"; BUILD_SCOPE="All"
fi
if [ "$ALL_FLAG" = "1" ]; then
    BUILD_SYSTEM="All"; BUILD_MODE="All"; BUILD_SCOPE="All"
fi

# ---------- Validate build parameters ----------
case "$BUILD_SYSTEM" in
    Windows|Linux|All) ;;
    *) echo "[build] Invalid system: '$BUILD_SYSTEM'. Valid: Windows, Linux, All" >&2; exit 1 ;;
esac
case "$BUILD_MODE" in
    Gui|Cli|All) ;;
    *) echo "[build] Invalid mode: '$BUILD_MODE'. Valid: Gui, Cli, All" >&2; exit 1 ;;
esac
case "$BUILD_SCOPE" in
    Frontend|Backend|All) ;;
    *) echo "[build] Invalid scope: '$BUILD_SCOPE'. Valid: Frontend, Backend, All" >&2; exit 1 ;;
esac

# ---------- Silent mode: default to full build when --build not given ----------
if [ "$SILENT" = "1" ]; then
    if [ "$BUILD_PARAM_SET" = "0" ]; then
        BUILD_SYSTEM="All"; BUILD_MODE="All"; BUILD_SCOPE="All"
    fi
    [ -z "$LANG_CODE" ] && LANG_CODE="EN"
fi

# ---------- Resolve language (interactive if not --silent and not --lang) ----------
if [ -z "$LANG_CODE" ]; then
    if [ "$SILENT" = "0" ] && [ "$BUILD_PARAM_SET" = "0" ]; then
        echo "=========================================="
        echo "$(m "LangTitle")"
        echo "=========================================="
        echo "1. ${MSG[LangOpt1]}"
        echo "2. ${MSG[LangOpt2]}"
        echo "3. ${MSG[LangOpt3]}"
        echo ""
        read -r -p "$(m "LangPrompt") " lang_choice
        case "$lang_choice" in
            2) LANG_CODE="CN" ;;
            3) LANG_CODE="RU" ;;
            *) echo "Defaulting to English..." ; LANG_CODE="EN" ;;
        esac
    else
        LANG_CODE="EN"
    fi
fi

# ---------- Interactive build menu (when no --build and not --silent) ----------
if [ "$BUILD_PARAM_SET" = "0" ] && [ "$SILENT" = "0" ]; then
    echo ""
    echo "=========================================="
    echo "$(m "MenuTitle")"
    echo "=========================================="
    echo ""

    # Install deps prompt
    if [ "$INSTALL_DEPS" = "0" ]; then
        read -r -p "$(m "DepPrompt") " deps_input
        case "$deps_input" in
            Y|y) INSTALL_DEPS=1 ;;
        esac
    fi

    echo ""
    echo "$(m "SelectTitle")"
    echo "$(m "Opt1")"
    echo "$(m "Opt2")"
    echo "$(m "Opt3")"
    echo "$(m "Opt4")"
    echo ""
    read -r -p "$(m "ChoicePrompt") " choice
    case "$choice" in
        1) BUILD_SYSTEM="Linux"; BUILD_MODE="Gui";  BUILD_SCOPE="All" ;;
        2) BUILD_SYSTEM="All";   BUILD_MODE="Cli";  BUILD_SCOPE="All" ;;
        3) BUILD_SYSTEM="All";   BUILD_MODE="All";  BUILD_SCOPE="All" ;;
        4) BUILD_SYSTEM="All";   BUILD_MODE="Cli";  BUILD_SCOPE="All"; ARCH_PARAM="all" ;;
        *) echo "$(m "Opt3") (default)"; BUILD_SYSTEM="All"; BUILD_MODE="All"; BUILD_SCOPE="All" ;;
    esac

    # CLI prompt (if GUI selected, ask whether to also build CLI)
    if [ "$BUILD_MODE" = "Gui" ]; then
        read -r -p "$(m "CliPrompt") " cli_input
        case "$cli_input" in
            Y|y) BUILD_MODE="All" ;;
        esac
    fi

    # Arch prompt
    if [ -z "$ARCH_PARAM" ]; then
        echo ""
        read -r -p "$(m "ArchPrompt") " arch_input
        case "$arch_input" in
            2) ARCH_PARAM="arm64" ;;
            3) ARCH_PARAM="x86" ;;
            *) ARCH_PARAM="x64" ;;
        esac
    fi
fi

# ---------- Normalize architecture ----------
# x86 only builds Windows CLI; Linux GUI and Darwin do not build x86
normalize_arch() {
    case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
        x64|amd64)         printf 'x64' ;;
        arm64|arm)         printf 'arm64' ;;
        x86|386|i386|i686) printf 'x86' ;;
        all)               printf 'all' ;;
        *)
            echo "[build] Unknown architecture: $1 (supported: x64/amd64, arm64/arm, x86/386, all)" >&2
            exit 1
            ;;
    esac
}

arch_to_goarch() {
    case "$1" in
        x64)   printf 'amd64' ;;
        arm64) printf 'arm64' ;;
        x86)   printf '386' ;;
    esac
}

# Default arch: "all" (amd64 + arm64 for CLI; current platform for GUI)
if [ -z "$ARCH_PARAM" ]; then
    ARCH_PARAM="all"
fi
BUILD_ARCH="$(normalize_arch "$ARCH_PARAM")"

# ---------- x86 restriction ----------
if [ "$BUILD_ARCH" = "x86" ]; then
    if [ "$BUILD_MODE" = "Gui" ] || [ "$BUILD_MODE" = "All" ]; then
        echo "[GUI] Warning: Linux GUI does not support x86, skipping GUI build" >&2
        if [ "$BUILD_MODE" = "All" ]; then
            BUILD_MODE="Cli"
        else
            BUILD_MODE="Cli"
        fi
    fi
    if [ "$BUILD_SYSTEM" = "Linux" ]; then
        echo "[build] Error: x86 only supports Windows CLI (--cli --arch x86)" >&2
        exit 1
    fi
fi

# ---------- Check Go is available ----------
command -v go >/dev/null 2>&1 || { echo "[build] Go not found, please install Go 1.25+" >&2; exit 1; }

# ---------- Compute derived flags (same logic as ps1) ----------
if [ "$BUILD_MODE" = "Gui" ] || [ "$BUILD_MODE" = "All" ]; then build_gui=1; else build_gui=0; fi
if [ "$BUILD_MODE" = "Cli" ] || [ "$BUILD_MODE" = "All" ]; then build_cli=1; else build_cli=0; fi
if { [ "$BUILD_SCOPE" = "Frontend" ] || [ "$BUILD_SCOPE" = "All" ]; } && [ "$build_gui" = "1" ]; then build_frontend=1; else build_frontend=0; fi
if { [ "$BUILD_SCOPE" = "Backend" ] || [ "$BUILD_SCOPE" = "All" ]; } && [ "$build_gui" = "1" ]; then build_backend=1; else build_backend=0; fi

# On Linux, "Windows" system means: only build CLI (can't build Windows GUI on Linux)
# "All" system means: build Linux GUI + cross-compile CLI for all platforms
if [ "$BUILD_SYSTEM" = "Windows" ]; then
    build_gui=0
    build_frontend=0
    build_backend=0
    build_cli=1
fi

echo ""
echo "=========================================="
echo " System=$BUILD_SYSTEM  Mode=$BUILD_MODE  Scope=$BUILD_SCOPE  Arch=$ARCH_PARAM"
echo "=========================================="

# ---------- Version from manifest (single source, shared GUI/CLI) ----------
MANIFEST_VERSION=""
MANIFEST_CHANNEL=""
if [ -f Package.appxmanifest ]; then
    MANIFEST_VERSION="$(grep -oP '<rel:Version>\K[^<]+' Package.appxmanifest 2>/dev/null | head -1 || true)"
    MANIFEST_CHANNEL="$(grep -oP '<rel:ReleaseChannel>\K[^<]+' Package.appxmanifest 2>/dev/null | head -1 || true)"
fi

# ---------- CLI Build (pure Go, no GUI deps, cross-compile all platforms) ----------
build_cli() {
    echo "=========================================="
    echo " SniShaper CLI Build (headless)"
    echo "   Version=$MANIFEST_VERSION Channel=$MANIFEST_CHANNEL"
    echo "=========================================="
    local OUT="build/bin/cli"
    mkdir -p "$OUT"
    local LDFLAGS="-s -w"
    if [ -n "$MANIFEST_VERSION" ]; then
        LDFLAGS="$LDFLAGS -X snishaper/app.buildVersion=$MANIFEST_VERSION"
    fi
    if [ -n "$MANIFEST_CHANNEL" ]; then
        LDFLAGS="$LDFLAGS -X snishaper/app.buildChannel=$MANIFEST_CHANNEL"
    fi

    # Determine platform list based on arch
    local platforms=()
    case "$BUILD_ARCH" in
        x64)   platforms=(windows/amd64 linux/amd64 darwin/amd64) ;;
        arm64) platforms=(windows/arm64 linux/arm64 darwin/arm64) ;;
        x86)   platforms=(windows/386)
               echo "[CLI] x86: only Windows binary built (Linux/Darwin skip x86)" ;;
        all)   platforms=(
                   windows/amd64 windows/arm64
                   linux/amd64   linux/arm64
                   darwin/amd64  darwin/arm64
               ) ;;
    esac

    for p in "${platforms[@]}"; do
        local goos="${p%/*}" goarch="${p#*/}"
        local name="snishaper-cli-$goos-$goarch"
        [ "$goos" = "windows" ] && name="$name.exe"
        echo "[CLI] building $goos/$goarch -> $OUT/$name"
        GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
            go build -tags "with_gvisor headless" -ldflags "$LDFLAGS" -o "$OUT/$name" ./cli
    done
    cp -r config rules "$OUT"/ 2>/dev/null || true
    echo "[CLI] Build complete: $OUT"
}

if [ "$build_cli" = "1" ]; then
    build_cli
    # If CLI-only, exit after CLI build
    if [ "$build_gui" = "0" ]; then
        echo ""
        echo "=========================================="
        echo " $(m "AllDone")"
        echo "=========================================="
        [ "$SILENT" = "0" ] && read -r -p "$(m "Exit") " || true
        exit 0
    fi
fi

# ---------- GUI Linux Build ----------
if [ "$build_gui" = "1" ]; then

export CGO_ENABLED=1
export GOOS="${GOOS:-linux}"

# ---------- GTK dependency check ----------
GTK_TAGS=()
if [ "$GTK3" = "1" ]; then
    GTK_TAGS+=("gtk3")
    command -v pkg-config >/dev/null 2>&1 || { echo "[build] pkg-config not found" >&2; exit 1; }
    pkg-config --exists gtk+-3.0 webkit2gtk-4.1 || {
        echo "[build] Missing GTK3 deps, install: libgtk-3-dev libwebkit2gtk-4.1-dev" >&2
        exit 1
    }
else
    command -v pkg-config >/dev/null 2>&1 || { echo "[build] pkg-config not found" >&2; exit 1; }
    pkg-config --exists gtk4 webkitgtk-6.0 || {
        echo "[build] Missing GTK4 deps, install: libgtk-4-dev libwebkitgtk-6.0-dev" >&2
        exit 1
    }
fi

# For GUI, use current platform arch (CGO required, can't cross-compile easily)
GUI_GOARCH="$(arch_to_goarch "$BUILD_ARCH")"
if [ "$BUILD_ARCH" = "all" ]; then
    GUI_GOARCH="$(go env GOARCH 2>/dev/null || echo amd64)"
fi
export GOARCH="$GUI_GOARCH"

echo "=========================================="
echo " SniShaper Linux GUI Build"
echo "   GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=1"
echo "   GTK: $([ ${#GTK_TAGS[@]} -gt 0 ] && echo "${GTK_TAGS[*]}" || echo "gtk4 (default)")"
echo "=========================================="

# ---------- 1. Frontend build (optional) ----------
if [ "$build_frontend" = "1" ]; then
    echo "$(m "FrontEnter")"
    if [ ! -d frontend ]; then
        echo "$(m "FrontErrDir")" >&2
        [ "$SILENT" = "0" ] && read -r -p "$(m "Exit") " || true
        exit 1
    fi
    cd frontend
    if [ "$INSTALL_DEPS" = "1" ]; then
        echo "$(m "FrontInstall")"
        npm install || { echo "$(m "FrontErrInstall")" >&2; exit 1; }
    fi
    echo "$(m "FrontBuild")"
    npm run build || { echo "$(m "FrontErrBuild")" >&2; exit 1; }
    echo "$(m "FrontDone")"
    cd ..
elif [ ! -f frontend/dist/index.html ]; then
    echo "[build] Warning: frontend/dist not found, run with --install-deps or --with-frontend" >&2
    [ "$SILENT" = "0" ] && read -r -p "$(m "Exit") " || true
    exit 1
fi

# ---------- 2. Go dependencies ----------
echo "$(m "BackStart")"
echo "$(m "BackInstallDeps")"
go mod download || { echo "$(m "BackErrInstallDeps")" >&2; exit 1; }

# ---------- 3. Version injection ----------
LDFLAGS="-s -w"
if [ -n "$MANIFEST_VERSION" ]; then
    LDFLAGS="$LDFLAGS -X snishaper/app.buildVersion=$MANIFEST_VERSION"
fi
if [ -n "$MANIFEST_CHANNEL" ]; then
    LDFLAGS="$LDFLAGS -X snishaper/app.buildChannel=$MANIFEST_CHANNEL"
fi
echo "$(m "BackBuildVersion" "$MANIFEST_VERSION")"

# ---------- 4. Compile ----------
OUT_DIR="build/bin"
OUT_BIN="$OUT_DIR/SniShaper"
mkdir -p "$OUT_DIR"

GOFLAGS=""
if [ ${#GTK_TAGS[@]} -gt 0 ]; then
    GOFLAGS="-tags ${GTK_TAGS[*]}"
fi
echo "[backend] go build (tags: ${GTK_TAGS[*]:-gtk4})..."
# shellcheck disable=SC2086
go build $GOFLAGS -ldflags "$LDFLAGS" -o "$OUT_BIN" . || {
    echo "$(m "BackErrBuild")" >&2
    [ "$SILENT" = "0" ] && read -r -p "$(m "Exit") " || true
    exit 1
}
echo "[backend] Build complete: $OUT_BIN"

# ---------- 5. Copy seed files ----------
echo "$(m "BackCopyCore")"
if [ -d rules ]; then
    mkdir -p "$OUT_DIR/rules"
    cp -r rules/* "$OUT_DIR/rules/" 2>/dev/null || true
fi
echo "$(m "BackCopyProxy")"
if [ -d config ]; then
    mkdir -p "$OUT_DIR/config"
    cp -r config/* "$OUT_DIR/config/" 2>/dev/null || true
fi
echo "$(m "BackDone")"

echo ""
echo "=========================================="
echo " Build OK: $OUT_BIN"
echo " Run: sudo $OUT_BIN  (TUN/system proxy needs root)"
echo "=========================================="

fi  # end GUI build

# ---------- Done ----------
echo ""
echo "=========================================="
echo " $(m "AllDone")"
echo "=========================================="

if [ "$SILENT" = "0" ]; then
    read -r -p "$(m "Exit") " || true
fi
