# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/SniShaper/SniShaper/wiki)
[![GitHub Release](https://img.shields.io/github/v/release/SniShaper/SniShaper?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub Downloads](https://img.shields.io/github/downloads/SniShaper/SniShaper/total?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&logo=git)](https://github.com/SniShaper/SniShaper/commits/main)
[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SniShaper/SniShaper/build.yml?style=flat-square&logo=githubactions&label=CI)](https://github.com/SniShaper/SniShaper/actions)

**SniShaper** is a local proxy tool designed for complex network environments, integrating **ECH Injection**, **TLS-RF Fragmentation**, **QUIC Obfuscation**, **Session Migration**, and other protocol stack technologies, paired with **TUN Virtual NIC** for full traffic takeover, delivering a stable and flexible browsing experience.

---

## Features

- **Multi-Mode Proxy**: MITM, Transparent, TLS-RF (TLS Fragmentation), QUIC, Migration (session persistence), Direct — covering diverse scenarios.
- **TUN Virtual NIC**: Native TUN support for transparent global traffic hijacking, auto-routing, and DNS hijacking.
- **ECH Injection**: Automatically fetches and injects ECH Config, with DoH discovery and hot-reload.
- **Smart Routing**: Auto-identifies blocked domains based on GFWList; auto-routing engine handles most sites without manual config.
- **Encrypted DNS**: Built-in anti-pollution DNS resolver with multi-node failover.
- **Cloudflare IP Pool**: Auto-speedtest, health check, and refresh.
- **NAT64 Support**: Flexible IP egress and service access.

---

## Quick Start

### 1. Run
Download the [latest version](https://github.com/SniShaper/SniShaper/releases) and run `snishaper.exe`. The app automatically requests admin elevation (required for TUN mode). If elevation fails, TUN is unavailable but other features work normally.

### 2. Certificate Reinstallation
Click "Certificate Management" -> "**Click to Reset Root Certificate**" in the main interface.

### 3. Configure and Start
The software includes a rich set of official rules. You can also customize your own rules in the "Rule Panel" based on actual conditions, and finally click "**Start Proxy**".

---

## Documentation

For more detailed technical principles, deployment tutorials, and customization guides, please refer to the [**GitHub Wiki**](https://github.com/SniShaper/SniShaper/wiki):

- **[Core Mode Introduction](https://github.com/SniShaper/SniShaper/wiki/Core-Proxy-Modes)**: Learn about the operation principles of TLS-RF, QUIC, and Server modes.
- **[Rule Customization Guide](https://github.com/SniShaper/SniShaper/wiki/Custom-Rules-Guide)**: Learn how to develop targeted rules.
- **[Interface Configuration Practice](https://github.com/SniShaper/SniShaper/wiki/GUI-Configuration)**: Learn how to quickly configure rules in the GUI.
- **[Common Troubleshooting](https://github.com/SniShaper/SniShaper/wiki/FAQ)**: Resolve certificate warnings, ineffective rules, and other common issues.

---

## Build and Development

This project is built based on **Wails v3**.

```powershell
# Clone the repository
git clone https://github.com/SniShaper/snishaper.git
cd snishaper

# Install frontend dependencies
cd frontend
npm install

# Build frontend static resources
npm run build
cd ..

# Complete the full compilation in one go (interactive mode)
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# Or with PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1

# Go main program compilation (script auto-runs go mod download)
go build -tags with_gvisor -ldflags="-s -w" -o "build/bin/snishaper.exe"
```

### Build Script Command-Line Parameters

`build_windows.ps1` supports the following parameters to skip interactive prompts:

| Parameter     | Values                         | Description                                                         |
| ------------- | ------------------------------ | ------------------------------------------------------------------- |
| `-Build`      | `frontend` / `backend` / `all` | Specify build target                                                |
| `-Lang`       | `en` / `cn` / `ru`             | Specify interface language                                          |
| `-InstallDeps` | No value (switch)              | Install frontend npm dependencies                                   |
| `-BuildMsix`  | No value (switch)              | Build MSIX installation package                                     |
| `-SkipSign`   | No value (switch)              | Skip MSIX signing, output file will have `unsigned_` prefix (requires `-BuildMsix`) |
| `-Silent`     | No value (switch)              | Silent mode, skip all interactive prompts                          |

**Usage examples:**

```powershell
# Build frontend only (Chinese interface)
.\build_windows.ps1 -Build frontend -Lang cn

# Build backend only (English interface)
.\build_windows.ps1 -Build backend -Lang en

# Build both frontend and backend, with dependency install
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

Development environment recommendations:

- `Go 1.25+`
- `Node.js 24+`
- `npm 11+`
- `gVisor` (required for TUN mode)

Build outputs:

- Frontend assets located at `frontend/dist`
- Executable located at `build/bin/snishaper.exe`

---
## Cross-platform
This program supports Windows and Linux platforms. For the Linux version, please refer to [Linux Version](https://github.com/dongzheyu/SniShaper-Linux/).

## Acknowledgements

This project has benefited from the inspiration of the following excellent open-source projects:

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

## Contributors

Thanks to the following contributors for their contributions to this repository:

| <a href="https://github.com/mechrevo"><img src="https://avatars.githubusercontent.com/mechrevo" width="40" height="40" style="border-radius: 50%;" alt="mechrevo" /></a> | <a href="https://github.com/dongzheyu"><img src="https://avatars.githubusercontent.com/dongzheyu" width="40" height="40" style="border-radius: 50%;" alt="dongzheyu" /></a> | <a href="https://github.com/JetCPP-dongle"><img src="https://avatars.githubusercontent.com/JetCPP-dongle" width="40" height="40" style="border-radius: 50%;" alt="JetCPP-dongle" /></a> |
| :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
|           [mechrevo](https://github.com/mechrevo)            |          [dongzheyu](https://github.com/dongzheyu)           |      [JetCPP-dongle](https://github.com/JetCPP-dongle)       |
## Star History

<a href="https://www.star-history.com/?repos=snishaper/snishaper&type=date&legend=top-left">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&theme=dark&legend=top-left" />
<source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left" />
<img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left" />
</picture>
</a>

---
## License

[MIT License](LICENSE)
