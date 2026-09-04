#!/usr/bin/env bash
#
# build.sh — SniShaper multi-platform / multi-architecture build script
#            (Linux, macOS, WSL). PowerShell twin: build_windows.ps1
#
# ---------------------------------------------------------------------------
# OUTPUT LAYOUT
# ---------------------------------------------------------------------------
#   build/bin/cli/Windows/{x64,x86,arm64}/snishaper.exe
#   build/bin/cli/Linux/{x64,arm64}/snishaper
#   build/bin/cli/Darwin/{x64,arm64}/snishaper
#   build/bin/gui/Windows/{x64,x86,arm64}/snishaper.exe
#   build/bin/gui/Linux/{x64,arm64}/SniShaper
#
#   Every target directory also receives the runtime seed folders config/ and
#   rules/. The GUI is never built for Darwin (Wails desktop shell supports
#   Windows and Linux only); x86 exists on Windows only.
#
# ---------------------------------------------------------------------------
# TARGET MATRIX — 12 artifacts
# ---------------------------------------------------------------------------
#   CLI : Windows x64/x86/arm64, Linux x64/arm64, Darwin x64/arm64  = 7
#   GUI : Windows x64/x86/arm64, Linux x64/arm64                    = 5
#
# ---------------------------------------------------------------------------
# USAGE
# ---------------------------------------------------------------------------
#   ./build.sh                                    interactive wizard
#   ./build.sh --help                             this help
#   ./build.sh --all                              all 12 targets
#   ./build.sh --type cli --all                   all 7 CLI targets
#   ./build.sh --platform linux --arch arm64 --type cli --type gui
#   ./build.sh --ci --platform linux --arch arm64 --type cli --type gui
#   ./build.sh --dry-run --all                    resolve + print, build nothing
#
# PARAMETERS
#   -p, --platform <name>   windows|linux|darwin|all   (repeatable)
#   -a, --arch <name>       x64|x86|arm64|all          (repeatable)
#   -t, --type <name>       cli|gui|all                (repeatable)
#       --all               every valid combination (highest priority)
#       --dry-run           resolve the target list and exit without building
#       --ci                CI mode. Native compilation only: never export a
#                           cross CC/CXX, never prompt. GUI ARM64 must run on
#                           a native ARM64 runner (ubuntu-24.04-arm, macos-14).
#       --strict-native     CI mode + restrict every target (including CLI) to
#                           the host architecture. Opt-in; by default the CLI
#                           cross-compiles because it is pure Go with
#                           CGO_ENABLED=0, so no toolchain is involved.
#       --cross             local development only: allow cross toolchains for
#                           the Linux GUI (aarch64-linux-gnu-gcc). Refused
#                           under --ci / --strict-native.
#       --install-deps      run npm install before the frontend build
#       --gtk3              build the Linux GUI against GTK3/WebKit2GTK-4.1
#       --wails             use the Wails CLI ("wails build -platform os/arch")
#                           instead of plain "go build"
#       --silent            never prompt
#   -h, --help
#
# LEGACY PARAMETERS (kept working for existing pipelines)
#   --build <system>,<mode>,<scope>   e.g. --build linux,gui,all
#   --cli  --gui  --with-frontend  --arch <arch>  --lang en|cn|ru  --silent
#
# ---------------------------------------------------------------------------
# CI: NATIVE ARM64 POLICY
# ---------------------------------------------------------------------------
#   linux/arm64   -> ubuntu-24.04-arm   (native ARM64 runner)
#   darwin/arm64  -> macos-14           (Apple Silicon runner)
#   windows/arm64 -> windows-latest     (Go emits windows/arm64 without cgo;
#                                        no cross toolchain is involved)
#   In --ci / --strict-native mode the script never exports CC or CXX, so a
#   build log can never contain "aarch64-linux-gnu-gcc" or "osxcross".
#
#   Example (native ARM64 Linux runner, CLI + GUI):
#     ./build.sh --ci --platform linux --arch arm64 --type cli --type gui
#
# ---------------------------------------------------------------------------
# VERIFICATION
# ---------------------------------------------------------------------------
#   file build/bin/cli/Linux/arm64/snishaper     -> ELF aarch64
#   file build/bin/gui/Linux/arm64/SniShaper     -> ELF aarch64
#   file build/bin/cli/Darwin/arm64/snishaper    -> Mach-O arm64
#
set -euo pipefail

cd "$(dirname "$0")"

# ---------- Go module proxy (overridable via env) ----------
if [ -z "${GOPROXY:-}" ]; then
    export GOPROXY="https://goproxy.cn,direct"
fi

# ===========================================================================
# i18n — plain functions (no associative arrays: /bin/bash on macOS is 3.2)
# ===========================================================================
msg_en() {
    case "$1" in
        LangTitle)      printf '%s' 'Please select your language / Выберите язык' ;;
        LangPrompt)     printf '%s' 'Enter your choice (1=English, 2=Chinese, 3=Russian)' ;;
        MenuTitle)      printf '%s' '       SniShaper Build Menu' ;;
        DepPrompt)      printf '%s' 'Install frontend npm dependencies? [y/N]' ;;
        TypeTitle)      printf '%s' 'Build type (comma separated, empty = all):' ;;
        TypeOpt1)       printf '%s' '  1) CLI  (headless, cross-platform)' ;;
        TypeOpt2)       printf '%s' '  2) GUI  (Windows + Linux desktop shell)' ;;
        TypeOpt3)       printf '%s' '  3) All' ;;
        PlatformTitle)  printf '%s' 'Platform (comma separated, empty = all):' ;;
        PlatformOpt1)   printf '%s' '  1) Windows' ;;
        PlatformOpt2)   printf '%s' '  2) Linux' ;;
        PlatformOpt3)   printf '%s' '  3) Darwin / macOS (CLI only)' ;;
        PlatformOpt4)   printf '%s' '  4) All' ;;
        ArchTitle)      printf '%s' 'Architecture (comma separated, empty = all):' ;;
        ArchOpt1)       printf '%s' '  1) x64' ;;
        ArchOpt2)       printf '%s' '  2) x86 (Windows only)' ;;
        ArchOpt3)       printf '%s' '  3) arm64' ;;
        ArchOpt4)       printf '%s' '  4) All' ;;
        CrossPrompt)    printf '%s' 'Allow cross-compilation toolchains for the Linux GUI? [y/N]' ;;
        ConfirmPrompt)  printf '%s' 'Start the build? [Y/n]' ;;
        NoTargets)      printf '%s' 'No valid target matched the selection.' ;;
        PlanTitle)      printf '%s' 'Build plan' ;;
        ColTarget)      printf '%s' 'TARGET' ;;
        ColState)       printf '%s' 'STATE' ;;
        ColNote)        printf '%s' 'NOTE' ;;
        DryRunNote)     printf '%s' '--dry-run: nothing was built.' ;;
        HostInfo)       printf '%s' 'Host: {0}/{1}   CI={2}   cross={3}' ;;
        VersionInfo)    printf '%s' 'Version: {0} (channel: {1})' ;;
        FrontEnter)     printf '%s' '[Frontend] Entering frontend directory...' ;;
        FrontErrDir)    printf '%s' '[Frontend] ERROR: failed to enter the frontend directory!' ;;
        FrontInstall)   printf '%s' '[Frontend] Installing npm dependencies...' ;;
        FrontErrInstall) printf '%s' '[Frontend] ERROR: npm install failed!' ;;
        FrontBuild)     printf '%s' '[Frontend] Running npm run build...' ;;
        FrontErrBuild)  printf '%s' '[Frontend] ERROR: npm run build failed!' ;;
        FrontDone)      printf '%s' '[Frontend] Frontend build completed successfully!' ;;
        FrontMissing)   printf '%s' '[Frontend] frontend/dist not found: pass --install-deps or run npm run build' ;;
        BackStart)      printf '%s' '[Backend] Starting Go build...' ;;
        BackInstallDeps) printf '%s' '[Backend] Installing Go dependencies...' ;;
        BackErrInstallDeps) printf '%s' '[Backend] ERROR: go mod download failed!' ;;
        BackBuildVersion) printf '%s' '[Backend] Build version: {0} (channel: {1})' ;;
        Start)          printf '%s' 'Starting build process...' ;;
        Done)           printf '%s' 'All selected tasks finished.' ;;
        Exit)           printf '%s' 'Press Enter to exit' ;;
        SummaryTitle)   printf '%s' 'Summary' ;;
        SummaryBuilt)   printf '%s' 'built  : {0}' ;;
        SummaryFailed)  printf '%s' 'failed : {0}' ;;
        SummarySkipped) printf '%s' 'skipped: {0}' ;;
    esac
}

msg_cn() {
    case "$1" in
        LangTitle)      printf '%s' '请选择语言' ;;
        LangPrompt)     printf '%s' '请输入选择 (1=English, 2=中文, 3=Русский)' ;;
        MenuTitle)      printf '%s' '       SniShaper 构建菜单' ;;
        DepPrompt)      printf '%s' '是否安装前端 npm 依赖？[y/N]' ;;
        TypeTitle)      printf '%s' '请选择构建类型（逗号分隔，留空=全部）：' ;;
        TypeOpt1)       printf '%s' '  1) CLI（headless，跨平台）' ;;
        TypeOpt2)       printf '%s' '  2) GUI（Windows + Linux 桌面端）' ;;
        TypeOpt3)       printf '%s' '  3) 全部' ;;
        PlatformTitle)  printf '%s' '请选择平台（逗号分隔，留空=全部）：' ;;
        PlatformOpt1)   printf '%s' '  1) Windows' ;;
        PlatformOpt2)   printf '%s' '  2) Linux' ;;
        PlatformOpt3)   printf '%s' '  3) Darwin / macOS（仅 CLI）' ;;
        PlatformOpt4)   printf '%s' '  4) 全部' ;;
        ArchTitle)      printf '%s' '请选择架构（逗号分隔，留空=全部）：' ;;
        ArchOpt1)       printf '%s' '  1) x64' ;;
        ArchOpt2)       printf '%s' '  2) x86（仅 Windows）' ;;
        ArchOpt3)       printf '%s' '  3) arm64' ;;
        ArchOpt4)       printf '%s' '  4) 全部' ;;
        CrossPrompt)    printf '%s' '是否允许 Linux GUI 使用交叉编译工具链？[y/N]' ;;
        ConfirmPrompt)  printf '%s' '确认开始构建？[Y/n]' ;;
        NoTargets)      printf '%s' '没有匹配到任何有效构建目标。' ;;
        PlanTitle)      printf '%s' '构建计划' ;;
        ColTarget)      printf '%s' '目标' ;;
        ColState)       printf '%s' '状态' ;;
        ColNote)        printf '%s' '说明' ;;
        DryRunNote)     printf '%s' '--dry-run：未执行任何构建。' ;;
        HostInfo)       printf '%s' '宿主机: {0}/{1}   CI={2}   cross={3}' ;;
        VersionInfo)    printf '%s' '版本：{0}（通道：{1}）' ;;
        FrontEnter)     printf '%s' '[前端] 正在进入 frontend 目录...' ;;
        FrontErrDir)    printf '%s' '[前端] 错误：无法进入 frontend 目录！' ;;
        FrontInstall)   printf '%s' '[前端] 正在安装 npm 依赖...' ;;
        FrontErrInstall) printf '%s' '[前端] 错误：npm install 失败！' ;;
        FrontBuild)     printf '%s' '[前端] 正在执行 npm run build...' ;;
        FrontErrBuild)  printf '%s' '[前端] 错误：npm run build 失败！' ;;
        FrontDone)      printf '%s' '[前端] 前端构建成功完成！' ;;
        FrontMissing)   printf '%s' '[前端] 未找到 frontend/dist：请加 --install-deps 或先执行 npm run build' ;;
        BackStart)      printf '%s' '[后端] 正在开始 Go 编译...' ;;
        BackInstallDeps) printf '%s' '[后端] 正在安装 Go 依赖...' ;;
        BackErrInstallDeps) printf '%s' '[后端] 错误：go mod download 失败！' ;;
        BackBuildVersion) printf '%s' '[后端] 构建版本：{0}（通道：{1}）' ;;
        Start)          printf '%s' '开始执行构建流程...' ;;
        Done)           printf '%s' '所有选定的任务已执行完毕。' ;;
        Exit)           printf '%s' '按回车键退出' ;;
        SummaryTitle)   printf '%s' '结果汇总' ;;
        SummaryBuilt)   printf '%s' '成功：{0}' ;;
        SummaryFailed)  printf '%s' '失败：{0}' ;;
        SummarySkipped) printf '%s' '跳过：{0}' ;;
    esac
}

msg_ru() {
    case "$1" in
        LangTitle)      printf '%s' 'Выберите язык' ;;
        LangPrompt)     printf '%s' 'Введите выбор (1=English, 2=中文, 3=Русский)' ;;
        MenuTitle)      printf '%s' '       Меню сборки SniShaper' ;;
        DepPrompt)      printf '%s' 'Установить npm зависимости фронтенда? [y/N]' ;;
        TypeTitle)      printf '%s' 'Тип сборки (через запятую, пусто = все):' ;;
        TypeOpt1)       printf '%s' '  1) CLI  (headless, кроссплатформенный)' ;;
        TypeOpt2)       printf '%s' '  2) GUI  (Windows + Linux)' ;;
        TypeOpt3)       printf '%s' '  3) Все' ;;
        PlatformTitle)  printf '%s' 'Платформа (через запятую, пусто = все):' ;;
        PlatformOpt1)   printf '%s' '  1) Windows' ;;
        PlatformOpt2)   printf '%s' '  2) Linux' ;;
        PlatformOpt3)   printf '%s' '  3) Darwin / macOS (только CLI)' ;;
        PlatformOpt4)   printf '%s' '  4) Все' ;;
        ArchTitle)      printf '%s' 'Архитектура (через запятую, пусто = все):' ;;
        ArchOpt1)       printf '%s' '  1) x64' ;;
        ArchOpt2)       printf '%s' '  2) x86 (только Windows)' ;;
        ArchOpt3)       printf '%s' '  3) arm64' ;;
        ArchOpt4)       printf '%s' '  4) Все' ;;
        CrossPrompt)    printf '%s' 'Разрешить кросс-тулчейн для Linux GUI? [y/N]' ;;
        ConfirmPrompt)  printf '%s' 'Начать сборку? [Y/n]' ;;
        NoTargets)      printf '%s' 'Не найдено ни одной подходящей цели сборки.' ;;
        PlanTitle)      printf '%s' 'План сборки' ;;
        ColTarget)      printf '%s' 'ЦЕЛЬ' ;;
        ColState)       printf '%s' 'СТАТУС' ;;
        ColNote)        printf '%s' 'ПРИМЕЧАНИЕ' ;;
        DryRunNote)     printf '%s' '--dry-run: сборка не выполнялась.' ;;
        HostInfo)       printf '%s' 'Хост: {0}/{1}   CI={2}   cross={3}' ;;
        VersionInfo)    printf '%s' 'Версия: {0} (канал: {1})' ;;
        FrontEnter)     printf '%s' '[Фронтенд] Переход в директорию frontend...' ;;
        FrontErrDir)    printf '%s' '[Фронтенд] ОШИБКА: не удалось войти в директорию frontend!' ;;
        FrontInstall)   printf '%s' '[Фронтенд] Установка npm зависимостей...' ;;
        FrontErrInstall) printf '%s' '[Фронтенд] ОШИБКА: npm install не удался!' ;;
        FrontBuild)     printf '%s' '[Фронтенд] Запуск npm run build...' ;;
        FrontErrBuild)  printf '%s' '[Фронтенд] ОШИБКА: npm run build не удался!' ;;
        FrontDone)      printf '%s' '[Фронтенд] Сборка фронтенда завершена успешно!' ;;
        FrontMissing)   printf '%s' '[Фронтенд] frontend/dist не найден: укажите --install-deps или выполните npm run build' ;;
        BackStart)      printf '%s' '[Бэкенд] Начало сборки Go...' ;;
        BackInstallDeps) printf '%s' '[Бэкенд] Установка Go зависимостей...' ;;
        BackErrInstallDeps) printf '%s' '[Бэкенд] ОШИБКА: go mod download не удался!' ;;
        BackBuildVersion) printf '%s' '[Бэкенд] Версия сборки: {0} (канал: {1})' ;;
        Start)          printf '%s' 'Начало сборки...' ;;
        Done)           printf '%s' 'Все выбранные задачи завершены.' ;;
        Exit)           printf '%s' 'Нажмите Enter для выхода' ;;
        SummaryTitle)   printf '%s' 'Итог' ;;
        SummaryBuilt)   printf '%s' 'успешно  : {0}' ;;
        SummaryFailed)  printf '%s' 'ошибки   : {0}' ;;
        SummarySkipped) printf '%s' 'пропущено: {0}' ;;
    esac
}

# m KEY [arg0] [arg1] [arg2] [arg3] — resolve a message for the active language
m() {
    _key="$1"
    _val=""
    case "${LANG_CODE:-EN}" in
        CN) _val="$(msg_cn "$_key")" ;;
        RU) _val="$(msg_ru "$_key")" ;;
        *)  _val="$(msg_en "$_key")" ;;
    esac
    if [ -z "$_val" ]; then
        _val="$_key"
    fi
    if [ "$#" -ge 2 ]; then
        _val="${_val//\{0\}/$2}"
    fi
    if [ "$#" -ge 3 ]; then
        _val="${_val//\{1\}/$3}"
    fi
    if [ "$#" -ge 4 ]; then
        _val="${_val//\{2\}/$4}"
    fi
    if [ "$#" -ge 5 ]; then
        _val="${_val//\{3\}/$5}"
    fi
    printf '%s' "$_val"
}

# ===========================================================================
# Target matrix helpers
# ===========================================================================
ALL_PLATFORMS="windows linux darwin"
ALL_TYPES="cli gui"

archs_for_platform() {
    case "$1" in
        windows) printf '%s' "x64 x86 arm64" ;;
        linux)   printf '%s' "x64 arm64" ;;
        darwin)  printf '%s' "x64 arm64" ;;
    esac
}

gui_platform_supported() {
    case "$1" in
        windows|linux) return 0 ;;
        *)             return 1 ;;
    esac
}

goarch_of() {
    case "$1" in
        x64)   printf '%s' "amd64" ;;
        arm64) printf '%s' "arm64" ;;
        x86)   printf '%s' "386" ;;
        *)     printf '%s' "$1" ;;
    esac
}

platform_dir() {
    case "$1" in
        windows) printf '%s' "Windows" ;;
        linux)   printf '%s' "Linux" ;;
        darwin)  printf '%s' "Darwin" ;;
        *)       printf '%s' "$1" ;;
    esac
}

is_valid_combo() {
    _t="$1"; _p="$2"; _a="$3"
    case " $(archs_for_platform "$_p") " in
        *" $_a "*) ;;
        *) return 1 ;;
    esac
    if [ "$_t" = "gui" ] && ! gui_platform_supported "$_p"; then
        return 1
    fi
    return 0
}

# get_valid_combinations — prints "type platform arch" for every valid target
get_valid_combinations() {
    for _t in $ALL_TYPES; do
        for _p in $ALL_PLATFORMS; do
            for _a in $(archs_for_platform "$_p"); do
                if is_valid_combo "$_t" "$_p" "$_a"; then
                    printf '%s %s %s\n' "$_t" "$_p" "$_a"
                fi
            done
        done
    done
}

# ===========================================================================
# Defaults
# ===========================================================================
LANG_CODE=""
PLATFORM_SEL=""
ARCH_SEL=""
TYPE_SEL=""
ALL_FLAG=0
DRY_RUN=0
CI_MODE=0
STRICT_NATIVE=0
CROSS_GUI=0
INSTALL_DEPS=0
GTK3=0
SILENT=0
WAILS_MODE=0
BUILD_FRONTEND=1
BUILD_BACKEND=1
SELECTION_GIVEN=0
LEGACY_BUILD=""
LEGACY_CLI=0
LEGACY_GUI=0
LEGACY_WF=0

# ===========================================================================
# Helpers
# ===========================================================================
usage() {
    awk 'NR>1 && /^set -euo pipefail$/ { exit } NR>1 { sub(/^# ?/, ""); print }' "$0"
}

die() {
    printf '[build] %s\n' "$1" >&2
    exit 1
}

append_unique() {
    _cur="$1"; _val="$2"
    case " $_cur " in
        *" $_val "*) printf '%s' "$_cur" ;;
        *)           printf '%s' "${_cur:+$_cur }$_val" ;;
    esac
}

normalize_platform() {
    case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
        windows|win)      printf '%s' "windows" ;;
        linux)            printf '%s' "linux" ;;
        darwin|macos|mac) printf '%s' "darwin" ;;
        all)              printf '%s' "all" ;;
    esac
}

normalize_arch() {
    case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
        x64|amd64|x86_64)  printf '%s' "x64" ;;
        arm64|aarch64)     printf '%s' "arm64" ;;
        x86|386|i386|i686) printf '%s' "x86" ;;
        all)               printf '%s' "all" ;;
    esac
}

normalize_type() {
    case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
        cli|headless) printf '%s' "cli" ;;
        gui|desktop)  printf '%s' "gui" ;;
        all)          printf '%s' "all" ;;
    esac
}

log()  { printf '%s\n' "$1"; }
warn() { printf '%s\n' "$1" >&2; }

# ===========================================================================
# Parameter parsing
# ===========================================================================
while [ "$#" -gt 0 ]; do
    case "$1" in
        -p|--platform)
            [ "$#" -ge 2 ] || die "--platform requires a value (windows|linux|darwin|all)"
            _v="$(normalize_platform "$2")"
            [ -n "$_v" ] || die "unknown platform: $2"
            PLATFORM_SEL="$(append_unique "$PLATFORM_SEL" "$_v")"
            SELECTION_GIVEN=1
            shift 2
            ;;
        -a|--arch)
            [ "$#" -ge 2 ] || die "--arch requires a value (x64|x86|arm64|all)"
            _v="$(normalize_arch "$2")"
            [ -n "$_v" ] || die "unknown architecture: $2"
            ARCH_SEL="$(append_unique "$ARCH_SEL" "$_v")"
            SELECTION_GIVEN=1
            shift 2
            ;;
        -t|--type)
            [ "$#" -ge 2 ] || die "--type requires a value (cli|gui|all)"
            _v="$(normalize_type "$2")"
            [ -n "$_v" ] || die "unknown type: $2"
            TYPE_SEL="$(append_unique "$TYPE_SEL" "$_v")"
            SELECTION_GIVEN=1
            shift 2
            ;;
        --all)           ALL_FLAG=1; SELECTION_GIVEN=1; shift ;;
        --dry-run)       DRY_RUN=1; shift ;;
        --ci)            CI_MODE=1; SILENT=1; shift ;;
        --strict-native) CI_MODE=1; STRICT_NATIVE=1; SILENT=1; shift ;;
        --cross)         CROSS_GUI=1; shift ;;
        --install-deps)  INSTALL_DEPS=1; shift ;;
        --gtk3)          GTK3=1; shift ;;
        --wails)         WAILS_MODE=1; shift ;;
        --silent)        SILENT=1; shift ;;
        --lang)
            [ "$#" -ge 2 ] || die "--lang requires a value (en|cn|ru)"
            case "$(printf '%s' "$2" | tr '[:lower:]' '[:upper:]')" in
                EN) LANG_CODE="EN" ;;
                CN) LANG_CODE="CN" ;;
                RU) LANG_CODE="RU" ;;
                *)  die "--lang must be en, cn or ru" ;;
            esac
            shift 2
            ;;
        --build)
            [ "$#" -ge 2 ] || die "--build requires a value (e.g. --build linux,gui,all)"
            LEGACY_BUILD="$2"
            SELECTION_GIVEN=1
            shift 2
            ;;
        --cli)           LEGACY_CLI=1; SELECTION_GIVEN=1; shift ;;
        --gui)           LEGACY_GUI=1; SELECTION_GIVEN=1; shift ;;
        --with-frontend) LEGACY_WF=1; INSTALL_DEPS=1; SELECTION_GIVEN=1; shift ;;
        -h|--help)       usage; exit 0 ;;
        *) die "unknown parameter: $1 (--help for usage)" ;;
    esac
done

# ---------- Legacy --build <system>,<mode>,<scope> mapping ----------
if [ -n "$LEGACY_BUILD" ]; then
    IFS=',' read -r _lsys _lmode _lscope <<EOF || true
$LEGACY_BUILD
EOF
    _lsys="$(printf '%s' "${_lsys:-}" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')"
    _lmode="$(printf '%s' "${_lmode:-}" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')"
    _lscope="$(printf '%s' "${_lscope:-}" | tr -d '[:space:]' | tr '[:upper:]' '[:lower:]')"
    case "$_lsys" in
        windows) PLATFORM_SEL="windows" ;;
        linux)   PLATFORM_SEL="linux" ;;
        all|"")  PLATFORM_SEL="all" ;;
        *)       die "invalid --build system: $_lsys" ;;
    esac
    case "$_lmode" in
        gui)    TYPE_SEL="gui" ;;
        cli)    TYPE_SEL="cli" ;;
        all|"") TYPE_SEL="all" ;;
        *)      die "invalid --build mode: $_lmode" ;;
    esac
    case "$_lscope" in
        frontend) BUILD_FRONTEND=1; BUILD_BACKEND=0 ;;
        backend)  BUILD_FRONTEND=0; BUILD_BACKEND=1 ;;
        all|"")   BUILD_FRONTEND=1; BUILD_BACKEND=1 ;;
        *)        die "invalid --build scope: $_lscope" ;;
    esac
fi

if [ "$LEGACY_CLI" = "1" ]; then
    TYPE_SEL="cli"
    if [ -z "$PLATFORM_SEL" ]; then
        PLATFORM_SEL="all"
    fi
fi
if [ "$LEGACY_GUI" = "1" ] || [ "$LEGACY_WF" = "1" ]; then
    TYPE_SEL="gui"
    PLATFORM_SEL="linux"
fi

if [ "$ALL_FLAG" = "1" ]; then
    # --all means "every platform and every architecture"; an explicit --type
    # still narrows the matrix (e.g. "--type cli --all" = all 7 CLI targets).
    PLATFORM_SEL="all"
    ARCH_SEL="all"
    if [ -z "$TYPE_SEL" ]; then
        TYPE_SEL="all"
    fi
fi

# ---------- CI never uses cross toolchains ----------
if [ "$CI_MODE" = "1" ] && [ "$CROSS_GUI" = "1" ]; then
    CROSS_GUI=0
fi

# ---------- Toolchain presence ----------
command -v go >/dev/null 2>&1 || die "Go not found, install Go 1.27+ (see go.mod)"

# ---------- Host detection ----------
HOST_OS="$(go env GOOS 2>/dev/null || true)"
if [ -z "$HOST_OS" ]; then
    case "$(uname -s)" in
        Linux*)               HOST_OS="linux" ;;
        Darwin*)              HOST_OS="darwin" ;;
        MINGW*|MSYS*|CYGWIN*) HOST_OS="windows" ;;
        *)                    HOST_OS="unknown" ;;
    esac
fi
HOST_GOARCH="$(go env GOARCH 2>/dev/null || true)"
if [ -z "$HOST_GOARCH" ]; then
    HOST_GOARCH="$(uname -m)"
fi
case "$HOST_GOARCH" in
    x86_64|amd64)  HOST_ARCH="x64" ;;
    aarch64|arm64) HOST_ARCH="arm64" ;;
    i386|i686|x86) HOST_ARCH="x86" ;;
    *)             HOST_ARCH="x64" ;;
esac

# ---------- Version from manifest (single source of truth) ----------
# portable sed: macOS ships BSD grep without -P, so no grep -oP here
manifest_tag() {
    if [ ! -f Package.appxmanifest ]; then
        return 0
    fi
    sed -n "s/.*<rel:$1>\([^<]*\)<\/rel:$1>.*/\1/p" Package.appxmanifest 2>/dev/null | head -1
}
MANIFEST_VERSION="$(manifest_tag Version)"
MANIFEST_CHANNEL="$(manifest_tag ReleaseChannel)"

# ===========================================================================
# Target resolution — lines of "type|platform|arch|state|note"
# ===========================================================================
target_state() {
    _t="$1"; _p="$2"; _a="$3"

    if ! is_valid_combo "$_t" "$_p" "$_a"; then
        printf '%s' "skip|not part of the 12-target matrix (GUI is Windows/Linux only)"
        return 0
    fi

    if [ "$_t" = "gui" ]; then
        if [ "$_p" = "darwin" ]; then
            printf '%s' "skip|GUI is not built for Darwin (Windows/Linux only)"
            return 0
        fi
        if [ "$HOST_OS" != "$_p" ]; then
            printf '%s' "skip|GUI needs a native $_p host (current host: $HOST_OS)"
            return 0
        fi
        if [ "$_p" = "linux" ] && [ "$HOST_ARCH" != "$_a" ]; then
            if [ "$CI_MODE" = "1" ]; then
                printf '%s' "skip|Linux GUI $_a needs a native $_a runner in CI"
                return 0
            fi
            if [ "$CROSS_GUI" != "1" ]; then
                printf '%s' "skip|Linux GUI $_a needs cgo/GTK: native only (--cross to override)"
                return 0
            fi
            if ! command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then
                printf '%s' "skip|aarch64-linux-gnu-gcc not installed"
                return 0
            fi
        fi
    fi

    if [ "$STRICT_NATIVE" = "1" ]; then
        if [ "$HOST_OS" != "$_p" ] || [ "$HOST_ARCH" != "$_a" ]; then
            printf '%s' "skip|--strict-native: $_p/$_a does not match host $HOST_OS/$HOST_ARCH"
            return 0
        fi
    fi

    if [ "$HOST_OS" = "$_p" ] && [ "$HOST_ARCH" = "$_a" ]; then
        printf '%s' "build|native host build"
        return 0
    fi
    printf '%s' "build|go cross build, CGO_ENABLED=0, no cross toolchain"
    return 0
}

list_matches() {
    # $1 = selected list, $2 = value, $3 = "all" allowance flag (unused)
    case " $1 " in
        *" $2 "*) return 0 ;;
        *" all "*) return 0 ;;
    esac
    if [ -z "$1" ]; then
        return 0
    fi
    return 1
}

resolve_targets() {
    RESOLVED=""
    for _t in $ALL_TYPES; do
        if ! list_matches "$TYPE_SEL" "$_t"; then
            continue
        fi
        for _p in $ALL_PLATFORMS; do
            if ! list_matches "$PLATFORM_SEL" "$_p"; then
                continue
            fi
            for _a in $(archs_for_platform "$_p"); do
                if ! list_matches "$ARCH_SEL" "$_a"; then
                    continue
                fi
                _st="$(target_state "$_t" "$_p" "$_a")"
                RESOLVED="${RESOLVED}${_t}|${_p}|${_a}|${_st}
"
            done
        done
    done
    printf '%s' "$RESOLVED"
}

print_plan() {
    printf '\n'
    printf '%s\n' "$(m PlanTitle)"
    printf '%s\n' '=============================================================================='
    printf '%-32s %-8s %s\n' "$(m ColTarget)" "$(m ColState)" "$(m ColNote)"
    printf '%s\n' '------------------------------------------------------------------------------'
    printf '%s\n' "$1" | while IFS='|' read -r _t _p _a _st _note; do
        if [ -z "${_t:-}" ]; then
            continue
        fi
        _label="$(printf '%s %s/%s' "$(printf '%s' "$_t" | tr '[:lower:]' '[:upper:]')" "$(platform_dir "$_p")" "$_a")"
        printf '%-32s %-8s %s\n' "$_label" "$_st" "$_note"
    done
    printf '%s\n' '=============================================================================='
}

# ===========================================================================
# Interactive wizard
# ===========================================================================
pick_from() {
    _title="$1"; shift
    printf '\n%s\n' "$_title"
    for _opt in "$@"; do
        printf '%s\n' "$_opt"
    done
    printf '> '
    read -r _answer || true
    printf '%s' "$_answer"
}

interactive_wizard() {
    printf '\n'
    printf '%s\n' '=========================================================='
    printf '%s\n' "$(m MenuTitle)"
    printf '%s\n' '=========================================================='

    if [ "$INSTALL_DEPS" = "0" ]; then
        printf '%s ' "$(m DepPrompt)"
        read -r _d || true
        case "$_d" in
            Y|y) INSTALL_DEPS=1 ;;
        esac
    fi

    _ans="$(pick_from "$(m TypeTitle)" "$(m TypeOpt1)" "$(m TypeOpt2)" "$(m TypeOpt3)")"
    case "$_ans" in
        1) TYPE_SEL="cli" ;;
        2) TYPE_SEL="gui" ;;
        *) TYPE_SEL="cli gui" ;;
    esac

    _ans="$(pick_from "$(m PlatformTitle)" "$(m PlatformOpt1)" "$(m PlatformOpt2)" "$(m PlatformOpt3)" "$(m PlatformOpt4)")"
    case "$_ans" in
        1) PLATFORM_SEL="windows" ;;
        2) PLATFORM_SEL="linux" ;;
        3) PLATFORM_SEL="darwin" ;;
        *) PLATFORM_SEL="windows linux darwin" ;;
    esac

    _ans="$(pick_from "$(m ArchTitle)" "$(m ArchOpt1)" "$(m ArchOpt2)" "$(m ArchOpt3)" "$(m ArchOpt4)")"
    case "$_ans" in
        1) ARCH_SEL="x64" ;;
        2) ARCH_SEL="x86" ;;
        3) ARCH_SEL="arm64" ;;
        *) ARCH_SEL="x64 x86 arm64" ;;
    esac

    printf '%s ' "$(m CrossPrompt)"
    read -r _c || true
    case "$_c" in
        Y|y) CROSS_GUI=1 ;;
        *)   CROSS_GUI=0 ;;
    esac

    print_plan "$(resolve_targets)"

    printf '%s ' "$(m ConfirmPrompt)"
    read -r _ok || true
    case "$_ok" in
        N|n) printf '%s\n' 'aborted.'; exit 0 ;;
    esac
}

# ===========================================================================
# Build engine
# ===========================================================================
GTK_CHECKED=0
check_gtk() {
    if [ "$GTK_CHECKED" = "1" ]; then
        return 0
    fi
    GTK_CHECKED=1
    command -v pkg-config >/dev/null 2>&1 || die "pkg-config not found (install libgtk-4-dev libwebkitgtk-6.0-dev)"
    if [ "$GTK3" = "1" ]; then
        pkg-config --exists gtk+-3.0 webkit2gtk-4.1 || die "missing GTK3 deps: libgtk-3-dev libwebkit2gtk-4.1-dev"
    else
        pkg-config --exists gtk4 webkitgtk-6.0 || die "missing GTK4 deps: libgtk-4-dev libwebkitgtk-6.0-dev"
    fi
}

ensure_frontend() {
    if [ "$BUILD_FRONTEND" = "0" ]; then
        if [ ! -f frontend/dist/index.html ]; then
            warn "$(m FrontMissing)"
            exit 1
        fi
        return 0
    fi

    log "$(m FrontEnter)"
    if [ ! -d frontend ]; then
        warn "$(m FrontErrDir)"
        exit 1
    fi
    ( cd frontend
      if [ "$INSTALL_DEPS" = "1" ]; then
          log "$(m FrontInstall)"
          npm install || { warn "$(m FrontErrInstall)"; exit 1; }
      fi
      log "$(m FrontBuild)"
      npm run build || { warn "$(m FrontErrBuild)"; exit 1; }
    ) || exit 1
    log "$(m FrontDone)"
}

copy_seed_folders() {
    if [ -d config ]; then
        mkdir -p "$1/config"
        cp -R config/. "$1/config/"
    fi
    if [ -d rules ]; then
        mkdir -p "$1/rules"
        cp -R rules/. "$1/rules/"
    fi
}

build_target() {
    _type="$1"; _plat="$2"; _arch="$3"
    _goos="$_plat"
    _goarch="$(goarch_of "$_arch")"
    _dir="build/bin/${_type}/$(platform_dir "$_plat")/${_arch}"
    _bin="snishaper"
    if [ "$_goos" = "windows" ]; then
        _bin="snishaper.exe"
    fi
    if [ "$_type" = "gui" ] && [ "$_goos" = "linux" ]; then
        _bin="SniShaper"
    fi
    _out="${_dir}/${_bin}"

    log ""
    log "[BUILD] $(printf '%s' "$_type" | tr '[:lower:]' '[:upper:]') $(platform_dir "$_plat") $_arch ($_goos/$_goarch) -> $_out"

    mkdir -p "$_dir"

    if [ "$_type" = "gui" ] && [ "$_goos" = "linux" ]; then
        check_gtk
    fi

    # ---- compiler selection -------------------------------------------------
    # The CLI is pure Go (CGO_ENABLED=0) and never needs a C toolchain.
    # The Linux GUI needs cgo + GTK/WebKit, so it builds with CGO_ENABLED=1
    # using the system compiler. A cross CC/CXX is exported only when the
    # operator explicitly passes --cross on a local machine; --ci and
    # --strict-native clear that flag, keeping ARM64 CI jobs toolchain-free.
    _cgo=0
    if [ "$_type" = "gui" ] && [ "$_goos" = "linux" ]; then
        _cgo=1
    fi

    _cc_args=""
    if [ "$_cgo" = "1" ] && [ "$HOST_ARCH" != "$_arch" ] && [ "$CROSS_GUI" = "1" ]; then
        _cc_args="CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++"
        log "[BUILD] cross toolchain: $_cc_args"
    fi

    # ---- tags / ldflags -----------------------------------------------------
    _tags="with_gvisor"
    if [ "$_type" = "cli" ]; then
        _tags="with_gvisor headless"
    fi
    if [ "$_type" = "gui" ] && [ "$_goos" = "linux" ] && [ "$GTK3" = "1" ]; then
        _tags="$_tags gtk3"
    fi

    _ldflags="-s -w"
    if [ "$_type" = "gui" ] && [ "$_goos" = "windows" ]; then
        _ldflags="$_ldflags -H windowsgui"
    fi
    if [ -n "$MANIFEST_VERSION" ]; then
        _ldflags="$_ldflags -X snishaper/app.buildVersion=$MANIFEST_VERSION"
    fi
    if [ -n "$MANIFEST_CHANNEL" ]; then
        _ldflags="$_ldflags -X snishaper/app.buildChannel=$MANIFEST_CHANNEL"
    fi

    # ---- compile ------------------------------------------------------------
    set +e
    if [ "$_type" = "cli" ]; then
        env GOOS="$_goos" GOARCH="$_goarch" CGO_ENABLED="$_cgo" $_cc_args \
            go build -tags "$_tags" -ldflags "$_ldflags" -o "$_out" ./cli
        _rc=$?
    else
        if [ "$WAILS_MODE" = "1" ]; then
            GOOS="$_goos" GOARCH="$_goarch" CGO_ENABLED="$_cgo" \
                wails build -platform "$_goos/$_goarch" -o "$_out"
            _rc=$?
        else
            env GOOS="$_goos" GOARCH="$_goarch" CGO_ENABLED="$_cgo" $_cc_args \
                go build -tags "$_tags" -ldflags "$_ldflags" -o "$_out" .
            _rc=$?
        fi
    fi
    set -e

    if [ "$_rc" -ne 0 ]; then
        warn "[BUILD] FAILED: $_type $(platform_dir "$_plat") $_arch (exit $_rc)"
        return 1
    fi

    copy_seed_folders "$_dir"
    log "[BUILD] OK: $_out"
    return 0
}

# ===========================================================================
# Main
# ===========================================================================
if [ "$SELECTION_GIVEN" = "0" ] && [ "$SILENT" = "0" ]; then
    if [ -z "$LANG_CODE" ]; then
        printf '%s\n' '=========================================================='
        printf '%s\n' "$(m LangTitle)"
        printf '%s\n' '=========================================================='
        printf '%s\n' '1. English'
        printf '%s\n' '2. Chinese'
        printf '%s\n' '3. Russian'
        printf '\n%s ' "$(m LangPrompt)"
        read -r _lc || true
        case "$_lc" in
            2) LANG_CODE="CN" ;;
            3) LANG_CODE="RU" ;;
            *) LANG_CODE="EN" ;;
        esac
    fi
    interactive_wizard
else
    if [ -z "$LANG_CODE" ]; then
        LANG_CODE="EN"
    fi
fi

if [ "$ALL_FLAG" = "1" ]; then
    PLATFORM_SEL="all"
    ARCH_SEL="all"
    if [ -z "$TYPE_SEL" ]; then
        TYPE_SEL="all"
    fi
fi

PLAN="$(resolve_targets)"
TARGET_COUNT="$(printf '%s' "$PLAN" | grep -c '|' || true)"
if [ "$TARGET_COUNT" -eq 0 ]; then
    warn "$(m NoTargets)"
    exit 1
fi

log ""
log "=========================================================="
log " SniShaper build"
log " $(m HostInfo "$HOST_OS" "$HOST_ARCH" "$CI_MODE" "$CROSS_GUI")"
log " $(m VersionInfo "$MANIFEST_VERSION" "$MANIFEST_CHANNEL")"
log "=========================================================="

print_plan "$PLAN"

if [ "$DRY_RUN" = "1" ]; then
    log ""
    log "$(m DryRunNote)"
    exit 0
fi

NEED_FRONTEND=0
if printf '%s' "$PLAN" | grep -q '^gui|.*|.*|build|'; then
    NEED_FRONTEND=1
fi

if [ "$BUILD_BACKEND" = "0" ]; then
    ensure_frontend
    log ""
    log "$(m Done)"
    exit 0
fi

log ""
log "$(m Start)"

if [ "$NEED_FRONTEND" = "1" ]; then
    ensure_frontend
fi

log ""
log "$(m BackStart)"
log "$(m BackInstallDeps)"
go mod download || { warn "$(m BackErrInstallDeps)"; exit 1; }
log "$(m BackBuildVersion "$MANIFEST_VERSION" "$MANIFEST_CHANNEL")"

BUILT_OK=0
BUILT_FAIL=0
BUILT_SKIP=0
FAILED_LIST=""

OLD_IFS="$IFS"
IFS='
'
for _line in $PLAN; do
    IFS="$OLD_IFS"
    if [ -z "$_line" ]; then
        IFS='
'
        continue
    fi
    _t="$(printf '%s' "$_line" | cut -d'|' -f1)"
    _p="$(printf '%s' "$_line" | cut -d'|' -f2)"
    _a="$(printf '%s' "$_line" | cut -d'|' -f3)"
    _st="$(printf '%s' "$_line" | cut -d'|' -f4)"
    _note="$(printf '%s' "$_line" | cut -d'|' -f5)"

    if [ "$_st" != "build" ]; then
        log "[SKIP]  $(printf '%s' "$_t" | tr '[:lower:]' '[:upper:]') $(platform_dir "$_p") $_a - $_note"
        BUILT_SKIP=$((BUILT_SKIP + 1))
    else
        if build_target "$_t" "$_p" "$_a"; then
            BUILT_OK=$((BUILT_OK + 1))
        else
            BUILT_FAIL=$((BUILT_FAIL + 1))
            FAILED_LIST="${FAILED_LIST}${_t} $(platform_dir "$_p") $_a
"
        fi
    fi
    IFS='
'
done
IFS="$OLD_IFS"

log ""
log "=========================================================="
log " $(m SummaryTitle)"
log " $(m SummaryBuilt "$BUILT_OK")"
log " $(m SummaryFailed "$BUILT_FAIL")"
log " $(m SummarySkipped "$BUILT_SKIP")"
log "=========================================================="

if [ -n "$FAILED_LIST" ]; then
    printf '%s' "$FAILED_LIST" >&2
fi

if [ "$BUILT_FAIL" -ne 0 ]; then
    exit 1
fi

log "$(m Done)"
if [ "$SILENT" = "0" ]; then
    read -r -p "$(m Exit) " || true
fi
