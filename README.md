# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go最低版本-1.27%2B-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/许可证-AGPL--3.0-blue?style=flat&logo=open-source-initiative)](LICENSE) [![Wiki](https://img.shields.io/badge/文档-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=版本)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=下载量)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=最后提交)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=持续集成)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** 是一款为复杂网络环境设计的本地代理软件，通过 **ECH 注入**、**TLS 分片**、**QUIC 转换**、**会话迁移** 等多种解决方案，帮助实现复杂环境下稳定而灵活的网络访问体验。

本项目提供跨平台支持。详见 **[Platform.md](docs/Platform.md)**。

---

## 姊妹项目：FlowWeaver

[**FlowWeaver**](https://github.com/SnishaperTeam/FlowWeaver) 是本项目的全功能分支，由原作者主导开发，两者**并行维护**：

| | SniShaper（本仓库） | [FlowWeaver](https://github.com/SnishaperTeam/FlowWeaver) |
|---|---|---|
| 定位 | 轻量级代理服务端 | 功能全面的客户端 / 网关 |
| 核心能力 | ECH 注入、TLS 分片、会话迁移 | Clash 订阅适配、全协议节点、WireGuard、TUN 网关 |

两者代码库独立，可各自演进；订阅与规则文件格式兼容。如果你需要**机场订阅导入、多协议节点选择或 WireGuard 隧道**，请使用 FlowWeaver。

---

## 交流群组

欢迎加入 QQ 群 **[Snishaper and FlowWeaver building](https://qm.qq.com/q/GtBOkAOiME)**，与 SniShaper 与 FlowWeaver 的用户和开发者直接交流、反馈问题与提交建议。

---

## 特性

- **多模式代理**：MITM（中间人）、Transparent（透传）、TLS-RF（TLS 分片）、QUIC、Migration（会话迁移）、Direct（直连）等多种模式覆盖不同网站的场景。
- **TUN 虚拟网卡**：全局流量透明劫持，自动路由与 DNS 劫持。
- **ECH 注入**：自动获取并注入 ECH Config，支持 DoH 发现与热更新。
- **智能分流**：基于 GFWList 自动识别被屏蔽域名，自动覆盖大量规则未包含网站。
- **加密 DNS**：内置抗污染 DNS 解析器，多节点故障转移稳定解析。
- **Cloudflare IP 优选池**：自动测速、健康检查与刷新。
- **NAT64 支持**：更灵活的 IP 出口，实现 IP 屏蔽下的服务访问。
- **进化模式**：自动测试多种规则组合，寻找目标站点的最优访问方式并一键应用。

---

## 快速开始

本项目起初主要支持 Windows，后续完成了 Linux 支持。现在同时提供 GUI 和 CLI。

对于一般用户，我们建议直接在 [Releases](https://github.com/SnishaperTeam/SniShaper/releases) 下载最新的 Windows 稳定版本。

对于其它平台的情况和构建方式，详见以下文档：

- **[Platform.md](docs/Platform.md)** — 各平台快速上手、CLI 用法与移动端伴侣。
- **[build.md](docs/build.md)** — 构建指南与 12 目标产物矩阵。

---

## 文档

想要了解更详细的技术原理和自定义规则指南，请参阅 [**GitHub Wiki**](https://github.com/SnishaperTeam/SniShaper/wiki)：

- **[核心模式介绍](https://github.com/SnishaperTeam/SniShaper/wiki/Core-Proxy-Modes)**：了解 TLS-RF、QUIC 与 Server 模式的运行原理。
- **[规则自定义指南](https://github.com/SnishaperTeam/SniShaper/wiki/Custom-Rules-Guide)**：了解如何开发针对性的规则。
- **[界面配置实操](https://github.com/SnishaperTeam/SniShaper/wiki/GUI-Configuration)**：了解在 GUI 快速配置规则。
- **[常见问题排除](https://github.com/SnishaperTeam/SniShaper/wiki/FAQ)**：解决证书警告、规则不生效等常见问题。
- **[Collaborator 协作条款](docs/COLLABORATOR_AGREEMENT.md)**：成为本仓库 collaborator 的条款、邀请与接受流程。

---

## 构建与开发

本项目在目前，前端基于 **Wails v3 + React 19 + MUI** 构建，核心使用 **Go** 开发，支持 Windows / Linux 双平台 GUI 与跨平台 CLI 共 12 个构建目标。完整的构建指南详见 **[build.md](docs/build.md)**。

我们会在不久后的另一个稳定版本完成原生 GUI 实现，减少前端的内存占用。

---

## 致谢

本项目受益于以下优秀开源项目的启发：

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

## 项目活跃度与贡献者

### 活跃度徽章

[![GitHub contributors](https://img.shields.io/github/contributors/SnishaperTeam/SniShaper?style=flat&label=总贡献者)](https://github.com/SnishaperTeam/SniShaper/graphs/contributors)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SnishaperTeam/SniShaper?style=flat&label=月均提交)](https://github.com/SnishaperTeam/SniShaper/graphs/contributors)
[![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&label=最近提交)](https://github.com/SnishaperTeam/SniShaper/commits/main)

### 综合活跃度趋势

<div align="center">
<a href="https://repobeats.axiom.co/" target="_blank">
<img src="https://repobeats.axiom.co/api/embed/f62c98a5231da45588ee71f26e3c1cc3f64edb6b.svg" alt="Repobeats analytics" />
</a>
</div>

### 贡献者图谱

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

## 许可

[GNU Affero General Public License v3.0](LICENSE)（AGPL-3.0）
