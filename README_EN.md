# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.27%2B-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat&logo=open-source-initiative)](LICENSE) [![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=Release)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=Downloads)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=Last%20Commit)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=CI)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** is a local proxy tool designed for complex network environments, integrating **ECH Injection**, **TLS Fragmentation**, **QUIC Obfuscation**, **Session Migration**, and other protocol stack technologies, paired with a **TUN Virtual NIC** for full traffic takeover, delivering a stable and flexible browsing experience.

This is a **dual-platform (Windows & Linux)** repository. Both platforms share the same codebase and versioning mechanism; platform-specific logic is isolated with Go build tags.

> Need a headless terminal version? This repository includes **SniShaper CLI** (`cli/` directory) — a cross-platform (Windows / Linux / macOS) headless build with a built-in TUI split-screen interface (real-time logs + command panel), retaining all core proxy capabilities and sharing the `Package.appxmanifest` version source with the GUI.

> Need an Android client? See **Lumine for Android** (<https://github.com/SniShaper/lumine-for-android>) — the mobile companion sharing the same routing concepts: a native Kotlin + Jetpack Compose (Material Design 3) UI with the Go (enimul) core bound via gomobile into a single AAR and no embedded WebView. It supports subscription management, rule editing, real-time logs and background keep-alive, and is also available on F-Droid (`com.moi.lumine`).

> Need a HarmonyOS client? See **Lumine for HarmonyOS** (<https://github.com/SnishaperTeam/lumine-for-harmonyos>) — the HarmonyOS companion with the same mobile vision: a native ArkTS + ArkUI interface (light & dark themes), a portable C++17 core accessed via NAPI as a single `liblumine_napi.so` with no embedded WebView; supporting VpnExtensionAbility (TUN) tunneling, local proxy listening (SOCKS5 / HTTP loopback inbound), subscription management, rule editing, real-time logs and running status notifications.

---

## Features

- **Multi-Mode Proxy**: MITM, Transparent, TLS-RF (TLS Fragmentation), QUIC, Migration (session persistence), Direct — covering diverse scenarios.
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

### CLI Version (Headless)

Don't need a GUI, or working in a server / SSH environment? This repository includes **SniShaper CLI** (`cli/` directory):

- **Three platforms**: Windows / Linux / macOS (amd64 + arm64).
- **TUI interface**: Upper pane for real-time scrolling proxy logs, lower pane for command input (supports Chinese aliases); logs never overwhelm your input.
- **Daemon mode**: `snishaper start` runs persistently; `status` / `stop` / `logs` / `proxy` / `sysproxy` / `tun` / `config` / `ca` subcommands for remote management.
- **Full core**: Shares the same proxy engine with the GUI (ECH injection, TLS fragmentation, QUIC, TUN/gvisor, GFWList routing, DoH, CF IP pool, NAT64, Evolution mode).
- **No auto-update**: Suitable for long-running server deployments.
- **Version consistency**: Shares `Package.appxmanifest` as the single version source with the GUI.

For build instructions, see **[build_EN.md — Artifact Matrix](build_EN.md#artifact-matrix-12-targets)**. Artifacts are organized under `build/bin/cli/<Platform>/<Arch>/` — just run the binary to enter the TUI.

> **Darwin / macOS CLI Notice:** The Darwin CLI currently does not have a dedicated macOS test machine for continuous real-world testing, so unexpected or currently unknown issues may occur. If you encounter any problems on Darwin / macOS, please report them promptly by opening an [Issue](https://github.com/SnishaperTeam/SniShaper/issues) or a [Pull Request](https://github.com/SnishaperTeam/SniShaper/pulls), so we can investigate and fix them.

### Certificate Re-install

In the main UI click **Certificate Management → Reset Root Certificate**. For the CLI version, use `snishaper ca regenerate` then `ca install`.

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

This project is built with **Wails v3 + React 19 + MUI**, with a **Go** backend, supporting Windows / Linux dual-platform GUI and cross-platform CLI for a total of 12 build targets. The full build guide has been split into **[build_EN.md](build_EN.md)**, covering:

- **[Artifact matrix](build_EN.md#artifact-matrix-12-targets)**: 12 targets with platform / arch / output paths, all build script parameters and examples.
- **[Windows Build](build_EN.md#windows-build)**: `build_windows.ps1` usage, MSIX packaging and behavior notes.
- **[Linux Build](build_EN.md#linux-build)**: GTK4 / GTK3 dependency installation and `build.sh` commands.
- **[Version & Release Channel](build_EN.md#version--release-channel)** / **[Development Environment](build_EN.md#development-environment)**: Version source and toolchain requirements.
- **[Continuous Integration](build_EN.md#continuous-integration)**: Dual-platform CI and release pipelines.
- **[Cross-Platform Notes](build_EN.md#cross-platform-notes)**: Go build tags and CLI subdirectory.

---

## Tools

### IP Scanner (tools/scanner.py)

A general-purpose IP scanning tool for discovering reachable proxy IPs for a target domain.

**Usage:**

```bash
python tools/scanner.py <domain:port> <CIDR> [max_threads]
```

**Parameters:**

| Parameter | Description | Example |
|-----------|-------------|---------|
| domain:port | Scan target | `open.spotify.com:443` |
| CIDR | IP range to scan | `35.186.224.0/24` |
| max_threads | Concurrent threads (default 64) | `128` |

**Examples:**

```bash
# Scan Spotify reachable IPs
python tools/scanner.py open.spotify.com:443 35.186.224.0/24 128

# Scan Google reachable IPs
python tools/scanner.py google.com:443 34.0.0.0/8 256

# Scan any domain
python tools/scanner.py example.com:80 1.0.0.0/16 64
```

**Output:**

- Results sorted by response time (fastest first)
- Logs saved in `tools/logs/` directory
- Valid IPs saved as `tools/logs/scan_*_valid.txt`

---

## Acknowledgements

This project has benefited from the inspiration of the following excellent open-source projects:

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

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

### Contributors Graph

<div align="center">
<a href="https://github.com/SnishaperTeam/SniShaper/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SnishaperTeam/SniShaper" alt="Contributors" />
</a>
</div>

---

## License

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
