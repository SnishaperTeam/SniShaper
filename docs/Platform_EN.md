# SniShaper Platforms & Quick Start

[中文](Platform.md) | [English](Platform_EN.md) | [Русский](Platform_RU.md)

> Back to [README](README_EN.md)

This project provides cross-platform support: **Windows and Linux** share the same codebase and versioning mechanism, with platform-specific logic isolated via Go build tags.

## Table of Contents

- [Quick Start](#quick-start)
  - [Windows](#windows)
  - [Linux](#linux)
  - [CLI Version (Headless)](#cli-version-headless)
  - [Certificate Re-install](#certificate-re-install)
  - [Configure and Start](#configure-and-start)
- [Mobile](#mobile)

---

## Quick Start

### Windows

Download `snishaper-windows-amd64.7z` (portable) or the MSIX installer from the [latest release](https://github.com/SnishaperTeam/SniShaper/releases), then extract / install and run `snishaper.exe`. The app requests admin elevation (required for TUN mode). If elevation fails, TUN is unavailable but other features work normally.

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

## Mobile

Need a mobile client? SniShaper has two companion versions built on the same routing concepts:

- **[Lumine for Android](https://github.com/Snishaper/lumine-for-android)** — a native Kotlin + Jetpack Compose (Material Design 3) UI with the Go (enimul) core bound via gomobile into a single AAR and no embedded WebView. It supports subscription management, rule editing, real-time logs and background keep-alive, and is also available on F-Droid (`com.moi.lumine`).
- **[Lumine for HarmonyOS](https://github.com/SnishaperTeam/lumine-for-harmonyos)** — a native ArkTS + ArkUI interface (light & dark themes), a portable C++17 core accessed via NAPI as a single `liblumine_napi.so` with no embedded WebView; supporting VpnExtensionAbility (TUN) tunneling, local proxy listening (SOCKS5 / HTTP loopback inbound), subscription management, rule editing, real-time logs and running status notifications.
