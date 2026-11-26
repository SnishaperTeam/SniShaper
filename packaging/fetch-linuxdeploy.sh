#!/usr/bin/env bash
# Fetch the linuxdeploy AppImage for one arch into packaging/tools/.
# The AppImage is a build tool, not a runtime dependency, so it is fetched
# on demand and never committed.
set -euo pipefail

arch="${1:?usage: fetch-linuxdeploy.sh <amd64|arm64>}"
mkdir -p packaging/tools
out="packaging/tools/linuxdeploy-${arch}.AppImage"

# Pinned release, never the moving `continuous` channel: an unpinned build
# tool is arbitrary remote code executed in CI. Digests come from the GitHub
# Releases API for tag 1-alpha-20251107-1. To upgrade: pick a new release
# tag, then update url base and both sha256 values (API: assets[].digest).
base="https://github.com/linuxdeploy/linuxdeploy/releases/download/1-alpha-20251107-1"
case "$arch" in
  amd64)
    url="${base}/linuxdeploy-x86_64.AppImage"
    sha256="c20cd71e3a4e3b80c3483cef793cda3f4e990aca14014d23c544ca3ce1270b4d"
    ;;
  arm64)
    url="${base}/linuxdeploy-aarch64.AppImage"
    sha256="620095110d693282b8ebeb244a95b5e911cf8f65f76c88b4b47d16ae6346fcff"
    ;;
  *) echo "unsupported arch: $arch" >&2; exit 1 ;;
esac

curl -fsSL "$url" -o "$out"
echo "${sha256}  ${out}" | sha256sum -c -
chmod +x "$out"
echo "fetched: $out"
