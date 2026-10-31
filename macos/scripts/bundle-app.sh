#!/usr/bin/env bash
# 打包 SniShaper.app（macOS 菜单栏应用）。
#
# 用法（在仓库根目录或 macos/ 目录下执行均可）:
#   macos/scripts/bundle-app.sh [--cli <snishaper-cli-path>] [--out <dir>]
#
# 步骤:
#   1. swift build -c release 编译菜单栏应用（SnishaperBar）
#   2. 组装 SniShaper.app:
#        Contents/MacOS/SnishaperBar              菜单栏应用
#        Contents/Resources/bin/snishaper         headless CLI（代理引擎）
#        Contents/Info.plist
#   3. ad-hoc 签名（codesign --sign -）：arm64 上完全未签名的二进制无法
#      通过 AMFI，ad-hoc 签名后可直接运行；未公证的 app 首次运行仍需在
#      系统设置中放行，见 docs/macos-app.md
#   4. zip 为 SniShaper-macos-<arch>.zip
#
# --cli 缺省时自动用 build.sh 构建当前架构的 headless CLI。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MACOS_DIR="$ROOT/macos"
OUT_DIR="$ROOT/build/bin/macos"
CLI_BIN=""

while [ $# -gt 0 ]; do
    case "$1" in
        --cli) CLI_BIN="$2"; shift 2 ;;
        --out) OUT_DIR="$2"; shift 2 ;;
        *) echo "未知参数: $1" >&2; exit 2 ;;
    esac
done

ARCH="$(uname -m)"

# build.sh 的产物目录用 x64/x86/arm64 命名，而 go env GOARCH 返回
# amd64/386/arm64 —— 必须归一化，否则 Intel 机器上找不到 CLI 产物。
goarch_normalize() {
    case "$1" in
        amd64|x86_64) printf '%s' "x64" ;;
        386)          printf '%s' "x86" ;;
        *)            printf '%s' "$1" ;;
    esac
}
CLI_ARCH="$(goarch_normalize "$(go env GOARCH)")"

# ---- 1. headless CLI -----------------------------------------------------
if [ -z "$CLI_BIN" ]; then
    echo "[bundle] building headless CLI ($ARCH)..."
    chmod +x "$ROOT/build.sh"
    "$ROOT/build.sh" --ci --platform darwin --arch "$CLI_ARCH" --type cli
    CLI_BIN="$ROOT/build/bin/cli/Darwin/$CLI_ARCH/snishaper"
fi
if [ ! -x "$CLI_BIN" ]; then
    echo "[bundle] CLI binary not found or not executable: $CLI_BIN" >&2
    exit 1
fi

# ---- 2. 菜单栏应用 -------------------------------------------------------
echo "[bundle] building SnishaperBar..."
cd "$MACOS_DIR"
swift build -c release --disable-sandbox
BAR_BIN="$MACOS_DIR/.build/release/SnishaperBar"

# ---- 3. 组装 .app --------------------------------------------------------
APP="$OUT_DIR/SniShaper.app"
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources/bin"
cp "$BAR_BIN" "$APP/Contents/MacOS/SnishaperBar"
cp "$CLI_BIN" "$APP/Contents/Resources/bin/snishaper"
chmod +x "$APP/Contents/MacOS/SnishaperBar" "$APP/Contents/Resources/bin/snishaper"

cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>SniShaper</string>
    <key>CFBundleDisplayName</key>
    <string>SniShaper</string>
    <key>CFBundleIdentifier</key>
    <string>com.snishaper.macos-bar</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleExecutable</key>
    <string>SnishaperBar</string>
    <key>LSMinimumSystemVersion</key>
    <string>13.0</string>
    <key>LSUIElement</key>
    <true/>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
PLIST

# ---- 4. ad-hoc 签名 -------------------------------------------------------
# arm64 macOS 拒绝运行完全未签名的二进制（AMFI），ad-hoc 签名（无身份，
# "signed to run locally"）让发布包可直接启动；公证（notarization）需要
# Apple Developer 账号，发布版暂不处理。
codesign --force --deep --sign - "$APP"
codesign --verify --strict "$APP"

# ---- 5. zip --------------------------------------------------------------
mkdir -p "$OUT_DIR"
ZIP="$OUT_DIR/SniShaper-macos-$ARCH.zip"
rm -f "$ZIP"
cd "$OUT_DIR"
ditto -c -k --keepParent SniShaper.app "$ZIP"

echo "[bundle] OK: $ZIP"
