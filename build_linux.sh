#!/usr/bin/env bash
#
# build_linux.sh — 在 Linux 本机构建 SniShaper（GTK4 / WebKitGTK 6.0）
#
# 用法:
#   ./build_linux.sh                # 仅编译后端（使用已有的 frontend/dist）
#   ./build_linux.sh --with-frontend  # 先构建前端再编译后端
#   ./build_linux.sh --gtk3          # 使用 GTK3 + webkit2gtk-4.1（默认 GTK4）
#
# 输出: build/bin/SniShaper（含 rules/ config/ 种子文件）
#
set -euo pipefail

cd "$(dirname "$0")"

GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
CGO_ENABLED=1
export GOOS GOARCH CGO_ENABLED

# Go 模块代理：默认 goproxy.cn（国内可达），可用环境变量覆盖。
# 例: GOPROXY=https://proxy.golang.org,direct ./build_linux.sh
if [ -z "${GOPROXY:-}" ]; then
    export GOPROXY="https://goproxy.cn,direct"
fi

BUILD_FRONTEND=0
GTK_TAGS=()

for arg in "$@"; do
    case "$arg" in
        --with-frontend) BUILD_FRONTEND=1 ;;
        --gtk3) GTK_TAGS+=("gtk3") ;;
        --help|-h)
            grep '^#' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *) echo "[build] 未知参数: $arg (--help 查看用法)" >&2; exit 1 ;;
    esac
done

echo "=========================================="
echo " SniShaper Linux 构建"
echo "   GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=1"
echo "   GTK: $([ ${#GTK_TAGS[@]} -gt 0 ] && echo "${GTK_TAGS[*]}" || echo "gtk4 (默认)")"
echo "=========================================="

# ---------- 0. 前置检查 ----------
command -v go >/dev/null 2>&1 || { echo "[build] 未找到 go，请先安装 Go 1.25+" >&2; exit 1; }
command -v pkg-config >/dev/null 2>&1 || { echo "[build] 未找到 pkg-config" >&2; exit 1; }

if [ ${#GTK_TAGS[@]} -gt 0 ]; then
    # GTK3 模式
    pkg-config --exists gtk+-3.0 webkit2gtk-4.1 || {
        echo "[build] 缺少 GTK3 依赖，请安装: libgtk-3-dev libwebkit2gtk-4.1-dev" >&2
        exit 1
    }
else
    # GTK4 模式（wails v3 默认）
    pkg-config --exists gtk4 webkitgtk-6.0 || {
        echo "[build] 缺少 GTK4 依赖，请安装: libgtk-4-dev libwebkitgtk-6.0-dev" >&2
        exit 1
    }
fi

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
    echo "[build] 警告: frontend/dist 不存在，运行 ./build_linux.sh --with-frontend 构建前端" >&2
    exit 1
fi

# ---------- 2. Go 依赖 ----------
echo "[backend] go mod download..."
go mod download

# ---------- 3. 版本号 ----------
# Package.appxmanifest 是唯一版本源（与 Windows 共用）：
#   <rel:Version>1.29.0</rel:Version>
#   <rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
VERSION=""
CHANNEL=""
if [ -f Package.appxmanifest ]; then
    VERSION="$(grep -oP '<rel:Version>\K[^<]+' Package.appxmanifest 2>/dev/null | head -1 || true)"
    CHANNEL="$(grep -oP '<rel:ReleaseChannel>\K[^<]+' Package.appxmanifest 2>/dev/null | head -1 || true)"
fi
LDFLAGS="-s -w"
if [ -n "$VERSION" ]; then
    LDFLAGS="$LDFLAGS -X snishaper/app.buildVersion=$VERSION"
fi
if [ -n "$CHANNEL" ]; then
    LDFLAGS="$LDFLAGS -X snishaper/app.buildChannel=$CHANNEL"
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
