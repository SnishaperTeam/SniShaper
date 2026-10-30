#!/usr/bin/env bash
# Fetch the linuxdeploy AppImage for one arch into packaging/tools/.
# The AppImage is a build tool, not a runtime dependency, so it is fetched
# on demand and never committed.
set -euo pipefail

arch="${1:?usage: fetch-linuxdeploy.sh <amd64|arm64>}"
mkdir -p packaging/tools
out="packaging/tools/linuxdeploy-${arch}.AppImage"

base="https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous"
case "$arch" in
  amd64) url="${base}/linuxdeploy-x86_64.AppImage" ;;
  arm64) url="${base}/linuxdeploy-aarch64.AppImage" ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

curl -fsSL "$url" -o "$out"
chmod +x "$out"
echo "fetched: $out"
