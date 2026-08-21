# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/SniShaper/SniShaper/wiki)
[![GitHub Release](https://img.shields.io/github/v/release/SniShaper/SniShaper?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub Downloads](https://img.shields.io/github/downloads/SniShaper/SniShaper/total?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&logo=git)](https://github.com/SniShaper/SniShaper/commits/main)
[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SniShaper/SniShaper/build.yml?style=flat-square&logo=githubactions&label=CI)](https://github.com/SniShaper/SniShaper/actions)

**SniShaper** is a local proxy tool designed for complex network environments, integrating **ECH Injection**, **TLS Fragmentation**, **QUIC Obfuscation**, **Session Migration**, and other protocol stack technologies, paired with a **TUN Virtual NIC** for full traffic takeover, delivering a stable and flexible browsing experience.

This is a **dual-platform (Windows & Linux)** repository. Both platforms share the same codebase and versioning mechanism; platform-specific logic is isolated with Go build tags.

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

Download `snishaper-windows-amd64.7z` (portable) or the MSIX installer from the [latest release](https://github.com/SniShaper/SniShaper/releases), then extract / install and run `snishaper.exe`. The app requests admin elevation (required for TUN mode). If elevation fails, TUN is unavailable but other features work normally.

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/en-us%20dark.svg" width="200"/>
</a>

### Linux

Download `snishaper-linux-amd64.tar.gz` from the [latest release](https://github.com/SniShaper/SniShaper/releases), then extract and run:

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

For detailed technical principles, deployment tutorials, and customization guides, refer to the [**GitHub Wiki**](https://github.com/SniShaper/SniShaper/wiki):

- **[Core Mode Introduction](https://github.com/SniShaper/SniShaper/wiki/Core-Proxy-Modes)**: Understand TLS-RF, QUIC and Server mode operation.
- **[Rule Customization Guide](https://github.com/SniShaper/SniShaper/wiki/Custom-Rules-Guide)**: Learn how to develop targeted rules.
- **[GUI Configuration Practice](https://github.com/SniShaper/SniShaper/wiki/GUI-Configuration)**: Quickly configure rules in the GUI.
- **[FAQ](https://github.com/SniShaper/SniShaper/wiki/FAQ)**: Resolve certificate warnings, rule issues and other common problems.

---

## Build and Development

This project is built with **Wails v3 + React 19 + MUI**, with a **Go** backend. The same repository produces both the Windows and Linux executables.

### Windows Build

```powershell
# Clone the repository
git clone https://github.com/SniShaper/SniShaper.git
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
| `-Build` | `frontend` / `backend` / `all` | Specify build target |
| `-Lang` | `en` / `cn` / `ru` | Specify interface language |
| `-InstallDeps` | No value (switch) | Install frontend npm dependencies |
| `-BuildMsix` | No value (switch) | Build MSIX installer package |
| `-SkipSign` | No value (switch) | Skip MSIX signing, output file gets `unsigned_` prefix (requires `-BuildMsix`) |
| `-Silent` | No value (switch) | Silent mode, skip all interactive prompts |

**Usage examples:**

```powershell
# Build frontend only (Chinese interface)
.\build_windows.ps1 -Build frontend -Lang cn

# Build backend only (English interface)
.\build_windows.ps1 -Build backend -Lang en

# Build both frontend and backend with dependency install
.\build_windows.ps1 -Build all -Lang cn -InstallDeps

# Build both and generate MSIX package (signed by default)
.\build_windows.ps1 -Build all -BuildMsix

# Build both and generate unsigned MSIX (skip signing)
.\build_windows.ps1 -Build all -BuildMsix -SkipSign

# Silent mode (for CI/CD, no interaction)
.\build_windows.ps1 -Silent

# Silent mode with build and packaging (skip signing)
.\build_windows.ps1 -Build all -Silent -BuildMsix -SkipSign

# No parameters = interactive mode (original behavior)
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
git clone https://github.com/SniShaper/SniShaper.git
cd SniShaper

# Compile backend only (uses existing frontend/dist)
./build_linux.sh

# Build frontend first, then backend
./build_linux.sh --with-frontend

# Use GTK3 + webkit2gtk-4.1
./build_linux.sh --gtk3
```

Build output is written to `build/bin/SniShaper` (including `rules/` and `config/` seed files). TUN / system proxy require root; run with `sudo ./build/bin/SniShaper`.

### Version & Release Channel

The version number and release channel (`release` / `beta` / `alpha` / `rc`) are **unified** in the root `Package.appxmanifest`:

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

Both the Windows and Linux builds read from this file and inject the values via ldflags (`snishaper/app.buildVersion`, `snishaper/app.buildChannel`). There is no separate version JSON file in the repository.

### Development Environment

- `Go 1.25+`
- `Node.js 24+` / `npm 11+`
- Windows: MSVC toolchain (Wails v3), WinApp CLI (MSIX packaging)
- Linux: GTK4 / WebKitGTK or GTK3 dev packages (see above)
- TUN mode depends on the gvisor network stack (enabled on Windows via the `with_gvisor` build tag)

Build outputs:

- Frontend assets at `frontend/dist`
- Windows executable at `build/bin/snishaper.exe`
- Linux executable at `build/bin/SniShaper`

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

[![GitHub contributors](https://img.shields.io/github/contributors/SniShaper/SniShaper?style=flat-square&label=Total Contributors)](https://github.com/SniShaper/SniShaper/graphs/contributors)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SniShaper/SniShaper?style=flat-square&label=Monthly Commits)](https://github.com/SniShaper/SniShaper/graphs/contributors)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&label=Last Commit)](https://github.com/SniShaper/SniShaper/commits/main)

### Activity Trend

<div align="center">
<a href="https://repobeats.axiom.co/" target="_blank">
<img src="https://repobeats.axiom.co/api/embed/f62c98a5231da45588ee71f26e3c1cc3f64edb6b.svg" alt="Repobeats analytics" />
</a>
</div>

### Core Contributors

<div align="center">
<a href="https://github.com/SniShaper/SniShaper/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SniShaper/SniShaper" alt="Contributors" />
</a>
</div>

---

## License

[MIT License](LICENSE)
