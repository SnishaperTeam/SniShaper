# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.27%2B-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat&logo=open-source-initiative)](LICENSE) [![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=Release)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=Downloads)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=Last%20Commit)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=CI)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** is a local proxy tool for complex network environments, using **ECH Injection**, **TLS Fragmentation**, **QUIC Conversion**, **Session Migration** and other solutions to deliver a stable and flexible browsing experience.

This project provides cross-platform support. See **[Platform_EN.md](docs/Platform_EN.md)**.

---

## Sister project: FlowWeaver

[**FlowWeaver**](https://github.com/SnishaperTeam/FlowWeaver) is the full-featured branch of this project, led by the same maintainer. Both are **actively maintained**:

| | SniShaper (this repository) | [FlowWeaver](https://github.com/SnishaperTeam/FlowWeaver) |
|---|---|---|
| Focus | Lightweight proxy server | Full-featured client / gateway |
| Core strengths | ECH injection, TLS fragmentation, session migration | Clash subscriptions, multi-protocol nodes, WireGuard, TUN gateway |

They keep separate codebases and evolve independently. Subscription and rule files are format-compatible. If you need **subscription import, multi-protocol node selection or WireGuard tunnels**, use FlowWeaver.

---

## Community

Join the QQ group **[Snishaper and FlowWeaver building](https://qm.qq.com/q/GtBOkAOiME)** to talk directly with SniShaper and FlowWeaver users and developers, report issues and send suggestions.

---

## Features

- **Multi-Mode Proxy**: MITM (man-in-the-middle), Transparent, TLS-RF (TLS fragmentation), QUIC, Migration (session migration), Direct — covering a wide range of site scenarios.
- **TUN Virtual NIC**: Transparent global traffic hijacking, auto-routing and DNS hijacking.
- **ECH Injection**: Automatically fetches and injects ECH Config, with DoH discovery and hot-reload.
- **Smart Routing**: Auto-identifies blocked domains based on GFWList, automatically covering many sites not included in the rules.
- **Encrypted DNS**: Built-in anti-pollution DNS resolver with multi-node failover for stable resolution.
- **Cloudflare IP Pool**: Auto speed-test, health check, and refresh.
- **NAT64 Support**: More flexible IP egress, enabling service access under IP blocking.
- **Evolution Mode**: Automatically tests combinations of rules to find the optimal access method for a target site and applies it with one click.

---

## Quick Start

The project started out Windows-only and later added Linux support. It now ships both a GUI and a CLI.

For most users, we recommend downloading the latest stable Windows build straight from [Releases](https://github.com/SnishaperTeam/SniShaper/releases).

For the other platforms and for build instructions, see the following documents:

- **[Platform_EN.md](docs/Platform_EN.md)** — per-platform quick start, CLI usage and mobile companions.
- **[build_EN.md](docs/build_EN.md)** — build guide and the 12-target artifact matrix.

---

## Documentation

For detailed technical principles and custom rule guides, refer to the [**GitHub Wiki**](https://github.com/SnishaperTeam/SniShaper/wiki):

- **[Core Mode Introduction](https://github.com/SnishaperTeam/SniShaper/wiki/Core-Proxy-Modes)**: Understand TLS-RF, QUIC and Server mode operation.
- **[Rule Customization Guide](https://github.com/SnishaperTeam/SniShaper/wiki/Custom-Rules-Guide)**: Learn how to develop targeted rules.
- **[GUI Configuration Practice](https://github.com/SnishaperTeam/SniShaper/wiki/GUI-Configuration)**: Quickly configure rules in the GUI.
- **[FAQ](https://github.com/SnishaperTeam/SniShaper/wiki/FAQ)**: Resolve certificate warnings, rule issues and other common problems.
- **[Collaborator Agreement](docs/COLLABORATOR_AGREEMENT.md)**: Terms, invitation and acceptance process for becoming a repository collaborator.

---

## Build and Development

The frontend is currently built with **Wails v3 + React 19 + MUI**, with the core developed in **Go**, supporting Windows / Linux dual-platform GUI and cross-platform CLI for a total of 12 build targets. The full build guide is in **[build_EN.md](docs/build_EN.md)**.

We will complete a native GUI implementation in an upcoming stable release, to reduce the frontend's memory footprint.

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

## Star History

<a href="https://www.star-history.com/?repos=snishaper%2Fsnishaper&type=timeline&logscale=&releases=&legend=bottom-right">
<picture>
<source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=timeline&theme=dark&logscale&legend=bottom-right" />
<source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=timeline&logscale&legend=bottom-right" />
<img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=timeline&logscale&legend=bottom-right" />
</picture>
</a>

---

## License

[GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
