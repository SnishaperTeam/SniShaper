#!/usr/bin/env bash
#
# build.sh — SniShaper 统一构建脚本（Unix / Linux / macOS）
#
# 用法:
#   ./build.sh                       # 交互模式（菜单选择 GUI / CLI / 全部）
#   ./build.sh --gui                 # GUI Linux（使用已有的 frontend/dist）
#   ./build.sh --with-frontend       # 先构建前端再编译 GUI 后端
#   ./build.sh --gtk3                # GUI 使用 GTK3 + webkit2gtk-4.1（默认 GTK4）
#   ./build.sh --cli                 # 仅构建 CLI 版（跨平台）
#   ./build.sh --all                 # GUI + CLI 全部构建
#   ./build.sh --arch x64            # 指定架构：x64（默认）/ arm64 / x86（仅 Windows）
#   ./build.sh --help                # 显示本帮助
#
# 输出:
#   GUI: build/bin/SniShaper（含 rules/ config/ 种子文件）
#   CLI: build/bin/cli/snishaper-cli-<os>-<arch>[.exe]（含 config/ rules/ 种子文件）
#
set -euo pipefail

cd "$(dirname "$0")"

GOOS="${GOOS:-linux}"
export GOOS

# BUILD_ARCH: x64 | arm64 | x86（x86 仅构建 Windows，Linux 含 CLI 与 Darwin 不构建 x86）
BUILD_ARCH=""

# Go 模块代理：默认 goproxy.cn（国内可达），可用环境变量覆盖。
# 例: GOPROXY=https://proxy.golang.org,direct ./build.sh
if [ -z "${GOPROXY:-}" ]; then
    export GOPROXY="https://goproxy.cn,direct"
fi

BUILD_FRONTEND=0
BUILD_CLI=0
BUILD_ALL=0
BUILD_GUI=0
INTERACTIVE=0
GTK_TAGS=()
BUILD_ARCH=""

# ---------- 架构归一化 ----------
# x86 仅构建 Windows；Linux（含 CLI）与 Darwin 不构建 x86
normalize_arch() {
    case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
        x64|amd64)         BUILD_ARCH="x64" ;;
        arm64|arm)         BUILD_ARCH="arm64" ;;
        x86|386|i386|i686) BUILD_ARCH="x86" ;;
        *)
            echo "[build] 未知架构: $1（支持 x64/amd64、arm64/arm、x86/386）" >&2
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

ARGS=("$@")
_i=0
while [ "$_i" -lt "${#ARGS[@]}" ]; do
    case "${ARGS[$_i]}" in
        --with-frontend) BUILD_FRONTEND=1 ;;
        --gtk3) GTK_TAGS+=("gtk3") ;;
        --cli) BUILD_CLI=1 ;;
        --gui) BUILD_GUI=1 ;;
        --all) BUILD_ALL=1 ;;
        --arch)
            _i=$((_i+1))
            if [ "$_i" -ge "${#ARGS[@]}" ]; then
                echo "[build] --arch 缺少参数值（x64/arm64/x86）" >&2
                exit 1
            fi
            normalize_arch "${ARGS[$_i]}"
            ;;
        --help|-h)
            grep '^#' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) echo "[build] 未知参数: ${ARGS[$_i]} (--help 查看用法)" >&2; exit 1 ;;
    esac
    _i=$((_i+1))
done

command -v go >/dev/null 2>&1 || { echo "[build] 未找到 go，请先安装 Go 1.25+" >&2; exit 1; }

# ---------- 交互模式 ----------
if [ "$BUILD_CLI" = "0" ] && [ "$BUILD_ALL" = "0" ] && [ "$BUILD_GUI" = "0" ] && [ $# -eq 0 ]; then
    INTERACTIVE=1
    echo "=========================================="
    echo " SniShaper 构建菜单"
    echo "=========================================="
    echo "1) GUI（Linux 桌面版）"
    echo "2) CLI（headless，跨平台）"
    echo "3) GUI + CLI"
    echo ""
    read -r -p "请选择 [1-3]: " choice
    case "$choice" in
        1) BUILD_GUI=1 ;;
        2) BUILD_CLI=1 ;;
        3) BUILD_ALL=1 ;;
        *) echo "[build] 无效选择: $choice" >&2; exit 1 ;;
    esac

    echo ""
    echo "选择目标架构（Linux GUI 仅支持 x64/arm64）："
    echo "1) x64（amd64，默认）"
    echo "2) arm64"
    echo "3) x86（仅 Windows CLI，Linux/Darwin 不构建）"
    read -r -p "请选择 [1-3]: " arch_choice
    case "$arch_choice" in
        2) BUILD_ARCH="arm64" ;;
        3) BUILD_ARCH="x86" ;;
        *) BUILD_ARCH="x64" ;;
    esac
fi

# 架构优先级: --arch 参数 > 交互选择 > GOARCH 环境变量 > amd64
if [ -z "$BUILD_ARCH" ]; then
    normalize_arch "${GOARCH:-amd64}"
fi
GOARCH="$(arch_to_goarch "$BUILD_ARCH")"
export GOARCH

# 无任何目标参数且非交互（CI 等）时，默认构建 GUI（与原 build_linux.sh 一致）
if [ "$BUILD_CLI" = "0" ] && [ "$BUILD_ALL" = "0" ] && [ "$BUILD_GUI" = "0" ]; then
    BUILD_GUI=1
fi

# ---------- x86 限制：Linux GUI 与 Darwin 不构建 x86 ----------
if [ "$BUILD_ARCH" = "x86" ]; then
    if [ "$BUILD_GUI" = "1" ]; then
        echo "[GUI] 警告: Linux GUI 不支持 x86，跳过 GUI 构建" >&2
        BUILD_GUI=0
    fi
    if [ "$BUILD_ALL" = "1" ]; then
        echo "[GUI] 警告: Linux GUI 不支持 x86，仅构建 Windows x86 CLI" >&2
        BUILD_ALL=0
        BUILD_CLI=1
    fi
    if [ "$BUILD_GUI" = "0" ] && [ "$BUILD_CLI" = "0" ] && [ "$BUILD_ALL" = "0" ]; then
        echo "[build] 错误: x86 架构仅支持 Windows CLI 目标（--cli --arch x86）" >&2
        exit 1
    fi
fi

# ---------- 版本号（Package.appxmanifest 为唯一版本源，GUI/CLI 共用） ----------
#   <rel:Version>1.29.0</rel:Version>
#   <rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
MANIFEST_VERSION=""
MANIFEST_CHANNEL=""
if [ -f Package.appxmanifest ]; then
    MANIFEST_VERSION="$(grep -oP '<rel:Version>\K[^<]+' Package.appxmanifest 2>/dev/null | head -1 || true)"
    MANIFEST_CHANNEL="$(grep -oP '<rel:ReleaseChannel>\K[^<]+' Package.appxmanifest 2>/dev/null | head -1 || true)"
fi

# ---------- CLI Build（纯 Go，无 GUI 依赖，交叉编译全平台） ----------
build_cli() {
    echo "=========================================="
    echo " SniShaper CLI 构建 (headless)"
    echo "   Version=$MANIFEST_VERSION Channel=$MANIFEST_CHANNEL"
    echo "=========================================="
    OUT="build/bin/cli"
    mkdir -p "$OUT"
    local LDFLAGS="-s -w"
    if [ -n "$MANIFEST_VERSION" ]; then
        LDFLAGS="$LDFLAGS -X snishaper/app.buildVersion=$MANIFEST_VERSION"
    fi
    if [ -n "$MANIFEST_CHANNEL" ]; then
        LDFLAGS="$LDFLAGS -X snishaper/app.buildChannel=$MANIFEST_CHANNEL"
    fi
    local platforms
    case "$BUILD_ARCH" in
        x64)   platforms=(windows/amd64 linux/amd64 darwin/amd64) ;;
        arm64) platforms=(windows/arm64 linux/arm64 darwin/arm64) ;;
        x86)   platforms=(windows/386)
               echo "[CLI] x86 仅构建 Windows（Linux/Darwin 不构建 x86）" ;;
    esac
    for p in "${platforms[@]}"; do
        local goos="${p%/*}" goarch="${p#*/}"
        local name="snishaper-cli-$goos-$goarch"
        [ "$goos" = "windows" ] && name="$name.exe"
        echo "[CLI] building $goos/$goarch -> $OUT/$name"
        GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
            go build -tags "with_gvisor headless" -ldflags "$LDFLAGS" -o "$OUT/$name" ./cli
    done
    cp -r config rules "$OUT"/
    echo "[CLI] 构建完成: $OUT"
}

if [ "$BUILD_CLI" = "1" ] || [ "$BUILD_ALL" = "1" ]; then
    build_cli
    if [ "$BUILD_CLI" = "1" ] && [ "$BUILD_ALL" = "0" ]; then
        exit 0
    fi
fi

# ---------- GUI Linux 构建 ----------
if [ "$BUILD_GUI" = "1" ] || [ "$BUILD_ALL" = "1" ]; then

CGO_ENABLED=1
export CGO_ENABLED

if [ ${#GTK_TAGS[@]} -gt 0 ]; then
    # GTK3 模式
    command -v pkg-config >/dev/null 2>&1 || { echo "[build] 未找到 pkg-config" >&2; exit 1; }
    pkg-config --exists gtk+-3.0 webkit2gtk-4.1 || {
        echo "[build] 缺少 GTK3 依赖，请安装: libgtk-3-dev libwebkit2gtk-4.1-dev" >&2
        exit 1
    }
else
    # GTK4 模式（wails v3 默认）
    command -v pkg-config >/dev/null 2>&1 || { echo "[build] 未找到 pkg-config" >&2; exit 1; }
    pkg-config --exists gtk4 webkitgtk-6.0 || {
        echo "[build] 缺少 GTK4 依赖，请安装: libgtk-4-dev libwebkitgtk-6.0-dev" >&2
        exit 1
    }
fi

echo "=========================================="
echo " SniShaper Linux GUI 构建"
echo "   GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=1"
echo "   Arch: $BUILD_ARCH"
echo "   GTK: $([ ${#GTK_TAGS[@]} -gt 0 ] && echo "${GTK_TAGS[*]}" || echo "gtk4 (默认)")"
echo "=========================================="

# ---------- 1. 前端构建（可选） ----------
if [ "$BUILD_FRONTEND" = "1" ]; then
    echo "[frontend] 构建前端..."
    if [ ! -d frontend ]; then
        echo "[build] 未找到 frontend 目录" >&2
        exit 1
    fi
    (cd frontend && npm install && npm run build)
    echo "[frontend] 构建完成"
elif [ ! -f frontend/dist/index.html ]; then
    echo "[build] 警告: frontend/dist 不存在，运行 ./build.sh --with-frontend 构建前端" >&2
    exit 1
fi

# ---------- 2. Go 依赖 ----------
echo "[backend] go mod download..."
go mod download

# ---------- 3. 版本号注入 ----------
LDFLAGS="-s -w"
if [ -n "$MANIFEST_VERSION" ]; then
    LDFLAGS="$LDFLAGS -X snishaper/app.buildVersion=$MANIFEST_VERSION"
fi
if [ -n "$MANIFEST_CHANNEL" ]; then
    LDFLAGS="$LDFLAGS -X snishaper/app.buildChannel=$MANIFEST_CHANNEL"
fi

# ---------- 4. 编译 ----------
OUT_DIR="build/bin"
OUT_BIN="$OUT_DIR/SniShaper"
mkdir -p "$OUT_DIR"

echo "[backend] go build (tags: ${GTK_TAGS[*]:-gtk4})..."
GOFLAGS=""
if [ ${#GTK_TAGS[@]} -gt 0 ]; then
    GOFLAGS="-tags ${GTK_TAGS[*]}"
fi
# shellcheck disable=SC2086
go build $GOFLAGS -ldflags "$LDFLAGS" -o "$OUT_BIN" .

echo "[backend] 编译完成: $OUT_BIN"

# ---------- 5. 拷贝运行种子文件 ----------
# 配置与规则首次运行时会种子化到 ~/.config/snishaper，
# 这里把打包默认值拷贝到 build/bin 下供 EnsureUserConfig 使用。
if [ -d rules ]; then
    mkdir -p "$OUT_DIR/rules"
    cp -r rules/* "$OUT_DIR/rules/" 2>/dev/null || true
    echo "[backend] 已拷贝 rules/ 种子"
fi
if [ -d config ]; then
    mkdir -p "$OUT_DIR/config"
    cp -r config/* "$OUT_DIR/config/" 2>/dev/null || true
    echo "[backend] 已拷贝 config/ 种子"
fi

echo ""
echo "=========================================="
echo " 构建成功: $OUT_BIN"
echo " 运行: sudo $OUT_BIN  (TUN/系统代理需要 root)"
echo "=========================================="

fi
