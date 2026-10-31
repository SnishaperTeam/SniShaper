#!/usr/bin/env bash
# Build Linux distribution packages (.deb / .rpm / AppImage) for one arch.
#
# Usage:
#   ./packaging/build-packages.sh <amd64|arm64> <version>
#
# Prerequisites (the CI workflow installs these; locally run build.sh first):
#   - go, nfpm (go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest)
#   - linuxdeploy AppImage (packaging/fetch-linuxdeploy.sh or set $LINUXDEPLOY)
#   - binaries already built by ./build.sh --platform linux --arch <x64|arm64> --type cli --type gui
#
# Outputs (build/dist/linux/<arch>/):
#   snishaper_<version>_amd64.deb     Debian package (GUI + CLI)
#   snishaper-<version>_<arch>.rpm    RPM package (GUI + CLI)
#   SniShaper-<version>-<arch>.AppImage  GUI AppImage (self-contained)
set -euo pipefail

arch="${1:?usage: build-packages.sh <amd64|arm64> <version>}"
version="${2:?usage: build-packages.sh <amd64|arm64> <version>}"

case "$arch" in
  amd64) path_arch=x64 ;;
  arm64) path_arch=arm64 ;;
  *) echo "unsupported arch: $arch (want amd64 or arm64)" >&2; exit 1 ;;
esac

gui_bin="build/bin/gui/Linux/${path_arch}/SniShaper"
cli_bin="build/bin/cli/Linux/${path_arch}/snishaper"
for f in "$gui_bin" "$cli_bin"; do
  if [ ! -f "$f" ]; then
    echo "missing binary: $f — run ./build.sh --platform linux --arch ${path_arch} --type cli --type gui first" >&2
    exit 1
  fi
done

if ! command -v nfpm >/dev/null 2>&1; then
  echo "nfpm not found — go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest" >&2
  exit 1
fi

outdir="build/dist/linux/${arch}"
mkdir -p "$outdir"

# 1) deb + rpm from the shared nfpm template.
sed -e "s/__ARCH__/${arch}/g" \
    -e "s/__PATH_ARCH__/${path_arch}/g" \
    -e "s/__VERSION__/${version}/g" \
    packaging/nfpm.yaml > "${outdir}/nfpm.yaml"

nfpm package -f "${outdir}/nfpm.yaml" -p deb -t "${outdir}/snishaper_${version}_${arch}.deb"
nfpm package -f "${outdir}/nfpm.yaml" -p rpm -t "${outdir}/snishaper-${version}_${arch}.rpm"
rm -f "${outdir}/nfpm.yaml"

# 2) AppImage via linuxdeploy (GUI only; the CLI ships as deb/rpm or archive).
# Best-effort: the deb/rpm pair above is the hard requirement, so a failed
# or unproven AppImage path (new linuxdeploy flags, plugin quirks) must not
# kill the release job.
linuxdeploy_bin="${LINUXDEPLOY:-packaging/tools/linuxdeploy-${arch}.AppImage}"
if [ ! -f "$linuxdeploy_bin" ]; then
  echo "linuxdeploy not found at ${linuxdeploy_bin} — run packaging/fetch-linuxdeploy.sh ${arch}" >&2
  echo "skipping AppImage (deb/rpm were built successfully)"
  exit 0
fi

appdir="$(mktemp -d)/SniShaper.AppDir"
mkdir -p "${appdir}/usr/bin"
cp "$gui_bin" "${appdir}/usr/bin/SniShaper"
cp packaging/snishaper.desktop "${appdir}/snishaper.desktop"
cp build/appicon.png "${appdir}/snishaper.png"

if ! OUTPUT="${outdir}/SniShaper-${version}-${arch}.AppImage" ARCH="$arch" "${linuxdeploy_bin}" \
  --appdir "$appdir" \
  --desktop-file "${appdir}/snishaper.desktop" \
  --icon-file "${appdir}/snishaper.png" \
  --output appimage; then
  echo "warning: AppImage build failed — shipping deb/rpm without AppImage" >&2
  exit 0
fi

echo "packages in ${outdir}:"
ls -l "$outdir"
