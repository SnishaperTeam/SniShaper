# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/License-MIT-blue?style=flat&logo=open-source-initiative)]() [![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=Release)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=Downloads)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=Last%20Commit)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=CI)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** is a local proxy tool designed for complex network environments, integrating **ECH Injection**, **TLS Fragmentation**, **QUIC Obfuscation**, **Session Migration**, and other protocol stack technologies, paired with a **TUN Virtual NIC** for full traffic takeover, delivering a stable and flexible browsing experience.

This is a **dual-platform (Windows & Linux)** repository. Both platforms share the same codebase and versioning mechanism; platform-specific logic is isolated with Go build tags.

> Need an Android client? See **Lumine for Android** (<https://github.com/SniShaper/lumine-for-android>) -- the mobile companion sharing the same routing concepts: a native Kotlin + Jetpack Compose (Material Design 3) UI with the Go (enimul) core bound via gomobile into a single AAR and no embedded WebView. It supports subscription management, rule editing, real-time logs and background keep-alive, and is also available on F-Droid (`com.moi.lumine`).

---

## Features

- **Multi-Mode Proxy**: MITM, Transparent, TLS-RF (TLS Fragmentation), QUIC, Migration (session persistence), Direct -- covering diverse scenarios.
- **TUN Virtual NIC**: WinTun on Windows and the gvisor network stack on Linux for transparent global traffic hijacking, auto-routing and DNS hijacking.
- **ECH Injection**: Automatically fetches and injects ECH Config, with DoH discovery and hot-reload.
- **Smart Routing**: Auto-identifies blocked domains based on GFWList; routing engine works without manual config.
- **Encrypted DNS**: Built-in anti-pollution DNS resolver with multi-node failover.
- **Cloudflare IP Pool**: Auto speed-test, health check, and refresh.
- **NAT64 Support**: Flexible IP egress and service access.
- **Evolution Mode**: Automatically tests combinations of rules to find the optimal access method for a target site and applies it with one click.

---

## Quick Start

### Windows

Download `snishaper-windows-amd64.7z` (portable) or the MSIX installer from the [latest release](https://github.com/SnishaperTeam/SniShaper/releases), then extract / install and run `snishaper.exe`. The app requests admin elevation (required for TUN mode). If elevation fails, TUN is unavailable but other features work normally.

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/en-us%20dark.svg" width="200"/>
</a>

### Linux

Download `snishaper-linux-amd64.tar.gz` from the [latest release](https://github.com/SnishaperTeam/SniShaper/releases), then extract and run:

```bash
tar -xzf snishaper-linux-amd64.tar.gz
sudo ./SniShaper
```

The app requests root privileges automatically (required for TUN mode). If elevation fails, TUN is unavailable but other features (proxy, etc.) work normally. The current build targets **amd64** and is based on **GTK4 + WebKitGTK 6.0** (GTK3 is also supported).

### Certificate Re-install

In the main UI click **Certificate Management -> Reset Root Certificate**.

### Configure and Start

The software includes a rich set of built-in rules. You can also customize rules in the **Rule Panel**, then click **Start Proxy**.

---

## Documentation

For detailed technical principles, deployment tutorials, and customization guides, refer to the [**GitHub Wiki**](https://github.com/SnishaperTeam/SniShaper/wiki):

- **[Core Mode Introduction](https://github.com/SnishaperTeam/SniShaper/wiki/Core-Proxy-Modes)**: Understand TLS-RF, QUIC and Server mode operation.
- **[Rule Customization Guide](https://github.com/SnishaperTeam/SniShaper/wiki/Custom-Rules-Guide)**: Learn how to develop targeted rules.
- **[GUI Configuration Practice](https://github.com/SnishaperTeam/SniShaper/wiki/GUI-Configuration)**: Quickly configure rules in the GUI.
- **[FAQ](https://github.com/SnishaperTeam/SniShaper/wiki/FAQ)**: Resolve certificate warnings, rule issues and other common problems.

---

## Build and Development

This project is built with **Wails v3 + React 19 + MUI**, with a **Go** backend. `build.sh` (Linux / macOS / WSL) and `build_windows.ps1` (Windows) share one target matrix and output layout.

### Artifact matrix (12 targets)

| Type | Platform | Arch | Output |
| --- | --- | --- | --- |
| CLI | Windows | `x64` / `x86` / `arm64` | `build/bin/cli/Windows/<arch>/snishaper.exe` |
| CLI | Linux | `x64` / `arm64` | `build/bin/cli/Linux/<arch>/snishaper` |
| CLI | Darwin | `x64` / `arm64` | `build/bin/cli/Darwin/<arch>/snishaper` |
| GUI | Windows | `x64` / `x86` / `arm64` | `build/bin/gui/Windows/<arch>/snishaper.exe` |
| GUI | Linux | `x64` / `arm64` | `build/bin/gui/Linux/<arch>/SniShaper` |

7 CLI + 5 GUI = **12 targets**; every target directory also carries the `config/` and `rules/` seed folders. GUI is never built for Darwin, and `x86` exists on Windows only. The GUI needs GTK/WebKit (Linux) and the frontend build, so it only builds on a same-OS native host; the CLI is pure Go (`CGO_ENABLED=0`) and all seven targets can be emitted from one runner without any cross toolchain.

Flags (`build.sh` / `build_windows.ps1`): `--platform` / `-Platform`, `--arch` / `-Arch`, `--type` / `-Type` (repeatable; PowerShell uses the comma-array form, e.g. `-Type cli,gui`), `--all` / `-All`, `--dry-run` / `-DryRun`, `--ci` / `-CI` (no prompts, never exports cross `CC`/`CXX`), `--cross` / `-Cross` (local only), `--install-deps` / `-InstallDeps`, `--gtk3` / `-Gtk3`, `--wails` / `-Wails`, `--silent` / `-Silent`, `--help` / `-Help`.

```bash
# Linux / macOS / WSL
./build.sh --all                                            # all 12 targets
./build.sh --type cli --all                                 # all 7 CLI targets
./build.sh --ci --platform linux --arch arm64 --type cli --type gui
./build.sh --dry-run --all                                  # plan only
```

```powershell
# Windows
.\build_windows.ps1 -All
.\build_windows.ps1 -Type cli -All
.\build_windows.ps1 -CI -Platform windows -Arch arm64 -Type cli,gui
.\build_windows.ps1 -Platform windows -Arch x64 -Type gui -InstallDeps -BuildMsix
```

**CI: ARM64 is built natively.** The regular CI workflow (build.yml, push/PR) only builds and smoke-tests - no packaging, no artifact uploads. The GUI matrix runs one (platform, arch) pair per job, so the ARM64 GUI is only compiled on native ARM runners: `linux/arm64` on `ubuntu-24.04-arm`, `linux/x64` on `ubuntu-latest`, and `windows/x64|x86|arm64` on `windows-latest` (Go emits `windows/arm64` with `CGO_ENABLED=0`; no cross toolchain). All seven CLI targets are pure Go (`CGO_ENABLED=0`) and are built once by the single `cli-build` job, so nothing is built twice. Packaging (MSIX / 7z / tar.gz) happens only in the release pipeline (tag or manual dispatch). Under `--ci` the scripts never export `CC`/`CXX`, so ARM64 logs cannot contain `aarch64-linux-gnu-gcc` or `osxcross`; the CI job greps the log and fails if either appears. For the Windows GUI version resource, it is regenerated per target architecture right before linking with go-winres (`--arch amd64/386/arm64`, emitted as `rsrc_windows_<arch>.syso`); no bare `.syso` is kept in the repository, so Linux/Darwin builds never link a Windows resource object.

Running with no selection flags opens the interactive wizard (language, type, platform, arch, optional cross toolchain, then confirmation of the plan). Verify artifacts with `file build/bin/gui/Linux/arm64/SniShaper` (ELF aarch64) or, on Windows, `dumpbin /headers ...` / the PE machine field.

### Windows Build

```powershell
# Clone the repository
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# Full compilation (interactive mode, auto-installs deps, optional MSIX)
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# Or with PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1
```

#### Build Script Command-Line Parameters

`build_windows.ps1` supports the following parameters to skip interactive prompts:

| Parameter | Values | Description |
| ------------- | ------------------------------ | ---------------------------------------------------------------- |
| `-Build` | `<system> <mode> <scope>` | Three-part build spec. **System**: `windows` / `linux` / `all`; **Mode**: `gui` / `cli` / `all`; **Scope**: `frontend` / `backend` / `all`. Omit for interactive menu; legacy `-Build frontend/backend/all` still works |
| `-Lang` | `en` / `cn` / `ru` | Prompt language, defaults to English |
| `-Arch` | `x64` / `arm64` / `x86` | Target architecture (accepts amd64/386 aliases), defaults to host. `x86` only builds Windows — Linux (incl. CLI) and Darwin are skipped |
| `-InstallDeps` | No value (switch) | Run `npm install` before the frontend build |
| `-BuildMsix` | No value (switch) | Build an MSIX installer after compilation (requires WinApp CLI) |
| `-SkipSign` | No value (switch) | Skip MSIX signing, output file gets `unsigned_` prefix (requires `-BuildMsix`) |
| `-Cli` | No value (switch) | Additionally build the headless CLI (target platforms determined by `-Arch`) into `build/bin/cli/<Platform>/<Arch>/` |
| `-Gtk3` | No value (switch) | Use GTK3 + webkit2gtk-4.1 for Linux (WSL) builds (equivalent to `build.sh --gtk3`) |
| `-Silent` | No value (switch) | Silent mode, skip all interactive prompts; defaults to `-Build windows gui all` and `-Lang en` when omitted |

**Behavior notes:**

- **Auto-elevation**: The script requires administrator privileges. When run as a regular user, it relaunches itself via a UAC prompt and passes all parameters through unchanged.
- **Pre-build cleanup**: Any running `snishaper` processes are force-terminated before the build to avoid file locks.
- **Version sync**: Before compiling the backend, the version and release channel are read from `Package.appxmanifest`, synced into the version resource via go-winres and injected via ldflags; if go-winres fails, the existing version resource is kept and the build continues. `go mod download` is always executed.
- **MSIX packaging**: Requires the WinApp CLI (`winget install Microsoft.WinAppCLI`); if the `devcert.pfx` certificate is missing, one is generated from the manifest and installed automatically. Output goes to the `Apppackage/` directory.
- **Linux build (WSL)**: `-Build linux` delegates to `build.sh` via WSL (GTK4 by default, `-Gtk3` switches to GTK3, `-Arch` forwarded as `--arch`); if WSL is not found, a warning is printed and the Linux build is skipped.

**Usage examples:**

```powershell
# Windows GUI, build everything (frontend + backend)
.\build_windows.ps1 -Build windows,gui,all

# Windows GUI, frontend only
.\build_windows.ps1 -Build windows,gui,frontend

# Linux GUI via WSL, build everything
.\build_windows.ps1 -Build linux,gui,all

# CLI only (headless, cross-platform)
.\build_windows.ps1 -Build windows,cli,all

# Both Windows + Linux GUI
.\build_windows.ps1 -Build all,gui,all

# Everything: all platforms, GUI + CLI
.\build_windows.ps1 -Build all,all,all

# arm64 build + MSIX packaging
.\build_windows.ps1 -Build windows,gui,all -Arch arm64 -BuildMsix

# Silent mode (for CI/CD, no interaction)
.\build_windows.ps1 -Silent

# Legacy format still works
.\build_windows.ps1 -Build frontend -Lang cn
.\build_windows.ps1 -Build all -BuildMsix -SkipSign

# No parameters = interactive mode
.\build_windows.ps1
```

### Linux Build

The Linux build uses `build_linux.sh` and runs on a Linux host (or WSL2 on Windows).

#### Dependencies (Ubuntu / Debian)

```bash
# GTK4 + WebKitGTK 6.0 (default)
sudo apt-get update
sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev

# Or use GTK3 + webkit2gtk-4.1
# sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

#### Build Commands

```bash
# Clone the repository
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# Interactive menu (1 GUI / 2 CLI / 3 GUI+CLI + architecture selection)
./build.sh

# GUI (uses existing frontend/dist)
./build.sh --gui

# Build frontend first, then backend
./build.sh --with-frontend

# Use GTK3 + webkit2gtk-4.1
./build.sh --gtk3

# CLI only (headless, cross-platform)
./build.sh --cli

# GUI + CLI
./build.sh --all

# Specify architecture (works for both GUI and CLI)
./build.sh --gui --arch arm64

# x86: Windows CLI only (Linux/Darwin skipped)
./build.sh --cli --arch x86
```

Build output is written to `build/bin/gui/Linux/<arch>/SniShaper` (including `rules/` and `config/` seed files). TUN / system proxy require root; run with `sudo ./build/bin/gui/Linux/x64/SniShaper`. CLI binaries land in `build/bin/cli/<Platform>/<Arch>/`.

### Version & Release Channel

The version number and release channel (`release` / `beta` / `alpha` / `rc`) are **unified** in the root `Package.appxmanifest`:

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

Both the Windows and Linux builds read from this file and inject the values via ldflags (`snishaper/app.buildVersion`, `snishaper/app.buildChannel`). There is no separate version JSON file in the repository.

### Development Environment

- `Go 1.27+`
- `Node.js 24+` / `npm 11+`
- Windows: MSVC toolchain (Wails v3), WinApp CLI (MSIX packaging)
- Linux: GTK4 / WebKitGTK or GTK3 dev packages (see above)
- TUN mode depends on the gvisor network stack (enabled on Windows via the `with_gvisor` build tag)

Build outputs:

- Frontend assets at `frontend/dist`
- Windows GUI at `build/bin/gui/Windows/<arch>/snishaper.exe` (x64 by default)
- Linux GUI at `build/bin/gui/Linux/<arch>/SniShaper`
- CLI at `build/bin/cli/{Windows,Linux,Darwin}/<arch>/snishaper[.exe]`

---

## Continuous Integration

Dual-platform CI pipelines:

- **`build.yml`**: Triggered on every push / PR. Builds Windows on `windows-2025` and Linux on `ubuntu-24.04`, then runs compilation and a binary smoke test.
- **`_release_pipeline.yml`**: Release pipeline. The Windows runner produces the MSIX and `snishaper-windows-amd64.7z` portable archive, the Ubuntu runner produces `snishaper-linux-amd64.tar.gz`, and finally the Windows runner merges both platform artifacts and creates the GitHub Release. Release notes are generated first by a local Ollama instance on the runner (default `qwen3.5:2b`); when Ollama is unavailable, it falls back to a categorized commit list.

---

## Cross-Platform Notes

Windows and Linux are built from the same repository, with platform-specific implementations isolated via Go build tags (e.g. `//go:build linux` / `windows`). There is no separate Linux repository to visit.

## Acknowledgements

This project has benefited from the inspiration of the following excellent open-source projects:

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

## Contributors

Thanks to the following contributors for their contributions to this repository:

| <a href="https://github.com/mechrevo"><img src="https://avatars.githubusercontent.com/mechrevo" width="40" height="40" style="border-radius: 50%;" alt="mechrevo" /></a> | <a href="https://github.com/dongzheyu"><img src="https://avatars.githubusercontent.com/dongzheyu" width="40" height="40" style="border-radius: 50%;" alt="dongzheyu" /></a> | <a href="https://github.com/JetCPP-dongle"><img src="https://avatars.githubusercontent.com/JetCPP-dongle" width="40" height="40" style="border-radius: 50%;" alt="JetCPP-dongle" /></a> |
| :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
| [mechrevo](https://github.com/mechrevo) | [dongzheyu](https://github.com/dongzheyu) | [JetCPP-dongle](https://github.com/JetCPP-dongle) |
| <a href="https://github.com/lzpls"><img src="https://avatars.githubusercontent.com/lzpls" width="40" height="40" style="border-radius: 50%;" alt="lzpls" /></a> |
| [lzpls](https://github.com/lzpls) |

## Star History

## Star History

<a href="https://www.star-history.com/?repos=snishaper%2Fsnishaper&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&theme=dark&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
 </picture>
</a>

---

## Project Activity & Contributors

### Activity Badges

[![GitHub contributors](https://img.shields.io/github/contributors/SnishaperTeam/SniShaper?style=flat&label=Total Contributors)](https://github.com/SnishaperTeam/SniShaper/graphs/contributors)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SnishaperTeam/SniShaper?style=flat&label=Monthly Commits)](https://github.com/SnishaperTeam/SniShaper/graphs/contributors)
[![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&label=Last Commit)](https://github.com/SnishaperTeam/SniShaper/commits/main)

### Activity Trend

<div align="center">
<a href="https://repobeats.axiom.co/" target="_blank">
<img src="https://repobeats.axiom.co/api/embed/f62c98a5231da45588ee71f26e3c1cc3f64edb6b.svg" alt="Repobeats analytics" />
</a>
</div>

### Core Contributors

<div align="center">
<a href="https://github.com/SnishaperTeam/SniShaper/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SnishaperTeam/SniShaper" alt="Contributors" />
</a>
</div>

---

## License

[MIT License](LICENSE)
