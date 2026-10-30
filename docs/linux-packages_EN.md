# Linux Packaging (deb / RPM / AppImage)

In addition to the existing archive distribution, SniShaper ships Linux distribution packages. Packaging definitions live in `packaging/`; the CI workflow is `.github/workflows/linux-packages.yml` (manual dispatch, never gates merges).

## Format evaluation

| Format | Verdict | Rationale |
| --- | --- | --- |
| `.deb` | ✅ provided | Largest coverage across Debian/Ubuntu and derivatives; one nfpm config, lowest maintenance cost |
| `.rpm` | ✅ provided | Fedora/RHEL/openSUSE ecosystem; shares the same nfpm config as deb, near-zero marginal cost |
| AppImage | ✅ provided | Install-free, single file, good for distros outside the package-manager coverage; GUI only (the CLI ships via deb/rpm or the archive) |
| Flatpak / Snap | ❌ not provided | Sandboxing conflicts with TUN/system-proxy capabilities (requires high-privilege interfaces and the host network stack); maintenance cost too high |

## Supported distributions

The GUI is Wails v3 (GTK4 + WebKitGTK 6.0); runtime dependencies:

| Distribution | Minimum | Runtime libraries |
| --- | --- | --- |
| Ubuntu | 24.04 LTS | `libgtk-4-1`, `libwebkitgtk-6.0-4` |
| Debian | 13 (trixie) | `libgtk-4-1`, `libwebkitgtk-6.0-4` |
| Fedora | 40+ | `gtk4`, `webkitgtk6.0` |

- The deb hard-depends on `libgtk-4-1`; WebKit is a recommend because its package name varies between releases (with it missing the GUI fails at launch with a clear message; the CLI is unaffected).
- RPM dependencies use Fedora names; openSUSE users should use the archive build or install the equivalent libraries themselves.
- The AppImage bundles its dependencies — no GTK/WebKit runtime needed — but **TUN mode still requires root** and `/dev/net/tun`.
- Architectures: `amd64` and `arm64`.
- Every format needs **administrator rights** to enable the system proxy and TUN; plain proxy mode (pointing a browser at 127.0.0.1:8080 manually) needs no elevation.

## Package contents (deb / rpm)

| File | Description |
| --- | --- |
| `/usr/bin/SniShaper` | GUI application |
| `/usr/bin/snishaper` | Headless CLI |
| `/usr/share/applications/snishaper.desktop` | Desktop entry (with zh_CN / ru localized comments) |
| `/usr/share/icons/hicolor/512x512/apps/snishaper.png` | Application icon |

## Upgrade and uninstall behavior

- **Upgrade**: `apt install ./snishaper_<version>_amd64.deb` (or `dnf upgrade ./snishaper-<version>.rpm`) replaces the package-owned files under `/usr` in place; no uninstall needed.
- **Uninstall**: `apt remove snishaper` / `dnf remove snishaper` removes only package-owned files.
- **User data is never touched by the package manager**: configuration, logs, rules and certificates all live in `~/.config/snishaper`, untouched by upgrades and removals; delete that directory manually once you are sure you no longer need it.
- The AppImage is a single file: deleting it uninstalls the app; `~/.config/snishaper` is likewise untouched.

## Building locally

```bash
# Prerequisites: go, node, GTK4/WebKitGTK dev packages (see docs/build_EN.md)
./build.sh --platform linux --arch x64 --type cli --type gui
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
./packaging/fetch-linuxdeploy.sh amd64
./packaging/build-packages.sh amd64 <version>
# Output: .deb / .rpm / .AppImage under build/dist/linux/amd64/
```

CI: dispatch the `linux-packages` workflow from the Actions tab; it produces per-architecture artifacts and verifies the deb contents (binaries, desktop entry, icon) automatically.
