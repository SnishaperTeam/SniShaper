[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

# SniShaper

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/coolapijust/snishaper/wiki)

**SniShaper** is a local proxy tool designed for complex network environments. It integrates multiple technologies, including **ECH Injection**, **TLS-RF Fragmentation**, **QUIC Connection Rebuilding**, and **Lightweight Server Mode Relay**, aiming to provide users with a stable access experience.

---

## Features

- **Six-mode comprehensive coverage**: Supports everything from lightweight `transparent` to advanced `server` forwarding to meet different needs.
- **Flexible strategies**:
  - **TLS-RF (TLS Fragmentation)**: Bypasses precise SNI-based blocking through fragmentation.
  - **QUIC Replay**: Bypasses conventional SNI detection using quic-go's obfuscation features.
  - **ECH Injection**: Automatically fetches and injects echconfig.
- **IP Optimization and WARP**: Integrated Cloudflare IP pool optimization and WARP Masque tunnel.
- **Smart Routing**: Automatically identifying blocked domains based on GFWList, allowing connection to most sites outside rules without manual configuration.

---

## Quick Start

### 1. Run
Download the [latest version](https://github.com/coolapijust/snishaper/releases) and run `snishaper.exe`.

### 2. Certificate Reinstallation
Click "Certificate Management" -> "**Click to Reinstall Certificate**" in the main interface.

### 3. Configure and Start
The software includes a rich set of official rules. You can also customize your own rules in the "Rule Panel" based on actual conditions, and finally click "**Start Proxy**".

---

## Documentation

For more detailed technical principles, deployment tutorials, and customization guides, please refer to the [**GitHub Wiki**](https://github.com/coolapijust/snishaper/wiki):

- **[Core Mode Introduction](https://github.com/coolapijust/snishaper/wiki/Core-Proxy-Modes)**: Learn about the operation principles of TLS-RF, QUIC, and Server modes.
- **[Rule Customization Guide](https://github.com/coolapijust/snishaper/wiki/Custom-Rules-Guide)**: Learn how to develop targeted rules.
- **[Interface Configuration Practice](https://github.com/coolapijust/snishaper/wiki/GUI-Configuration)**: Learn how to quickly configure rules in the GUI.
- **[Server Deployment](https://github.com/coolapijust/snishaper/wiki/Server-Deployment)**: Set up your own Server node on CF Workers or VPS.
- **[Common Troubleshooting](https://github.com/coolapijust/snishaper/wiki/FAQ)**: Resolve certificate warnings, ineffective rules, and other common issues.

---

## Build and Development

This project is built based on **Wails v3**.

```powershell
# Clone Repository
git clone https://github.com/coolapijust/snishaper.git
cd snishaper

# Install frontend dependencies
cd frontend
npm install

# Build frontend assets
npm run build
cd ..

# Build frontend and compile the GUI with gVisor real-TUN support
powershell -ExecutionPolicy Bypass -File .\scripts\build_windows.ps1
```

`snishaper.syso` is maintained in the repository, and the build script embeds the Windows icon/version metadata while building the Windows executable with the `with_gvisor` tag for real TUN support.

Recommended toolchain:

- `Go 1.25+`
- `Node.js 24+`
- `npm 11+`

Build outputs:

- Frontend assets: `frontend/dist`
- Executable: `build/bin/snishaper.exe`

---

## Acknowledgements

This project was inspired by and benefited from the following excellent open-source projects:

- [SNIBypassGUI](https://github.com/coolapijust/SniViewer)
- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)
- [usque](https://github.com/Diniboy1123/usque)

## License

[MIT License](LICENSE)
