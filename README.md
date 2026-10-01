# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go最低版本-1.27%2B-00ADD8?style=flat&logo=go)](https://golang.org) [![License](https://img.shields.io/badge/许可证-AGPL--3.0-blue?style=flat&logo=open-source-initiative)](LICENSE) [![Wiki](https://img.shields.io/badge/文档-Wiki-orange?style=flat&logo=readthedocs)](https://github.com/SnishaperTeam/SniShaper/wiki) [![GitHub Release](https://img.shields.io/github/v/release/SnishaperTeam/SniShaper?style=flat&logo=github&label=版本)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub Downloads](https://img.shields.io/github/downloads/SnishaperTeam/SniShaper/total?style=flat&logo=github&label=下载量)](https://github.com/SnishaperTeam/SniShaper/releases) [![GitHub last commit](https://img.shields.io/github/last-commit/SnishaperTeam/SniShaper?style=flat&logo=git&label=最后提交)](https://github.com/SnishaperTeam/SniShaper/commits/main) [![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SnishaperTeam/SniShaper/build.yml?style=flat&logo=githubactions&label=持续集成)](https://github.com/SnishaperTeam/SniShaper/actions)

**SniShaper** 是一款专为复杂网络环境设计的本地代理软件，通过 **ECH 注入**、**TLS 分片**、**QUIC 转换**、**会话迁移** 等多种协议栈技术，配合 **TUN 虚拟网卡** 接管全局流量，在复杂网络环境下提供稳定灵活的访问体验。

本项目为 **Windows 与 Linux 双平台** 仓库，共用同一套代码与版本机制，平台相关逻辑通过 Go build tags 隔离。

> 需要无图形界面的终端版本？本仓库内置 **SniShaper CLI**（`cli/` 目录）——跨平台（Windows / Linux / macOS）headless 版，内置 TUI 分屏界面（实时日志 + 命令面板），保留全部核心代理能力，与 GUI 共用 `Package.appxmanifest` 版本源。

> 需要 Android 移动端？请参见 **Lumine for Android**（<https://github.com/SniShaper/lumine-for-android>）——基于相同路由理念的移动端配套版本：Kotlin + Jetpack Compose（Material Design 3）原生界面，Go（enimul）核心经 gomobile 绑定为单个 AAR，无 WebView 内嵌；支持订阅管理、规则编辑、实时日志与后台保活，也可通过 F-Droid（`com.moi.lumine`）获取。

> 需要 HarmonyOS 移动端？请参见 **Lumine for HarmonyOS**（<https://github.com/SnishaperTeam/lumine-for-harmonyos>）——同一移动端理念的鸿蒙配套版本：ArkTS + ArkUI 原生界面（深浅色），便携 C++17 核心经 NAPI 接入为单个 `liblumine_napi.so`，无 WebView 内嵌；支持 VpnExtensionAbility（TUN）隧道、本地代理监听（SOCKS5 / HTTP 回环入站）、订阅管理、规则编辑、实时日志与运行状态通知。

---

## 特性

- **多模式代理**：MITM（中间人）、Transparent（透传）、TLS-RF（TLS 分片）、QUIC、Migration（会话迁移）、Direct（直连）等多种模式覆盖不同场景。
- **TUN 虚拟网卡**：Windows 走 WinTun、Linux 走 gvisor 网络栈，全局流量透明劫持，自动路由与 DNS 劫持。
- **ECH 注入**：自动获取并注入 ECH Config，支持 DoH 发现与热更新。
- **智能分流**：基于 GFWList 自动识别被屏蔽域名，自动路由引擎无需手动配置即可分流。
- **加密 DNS**：内置抗污染 DNS 解析器，支持多节点故障转移。
- **Cloudflare IP 优选池**：自动测速、健康检查与刷新。
- **NAT64 支持**：更灵活的 IP 出口和服务访问。
- **进化模式（Evolution）**：自动测试多种规则组合，寻找目标站点的最优访问方式并一键应用。

---

## 快速开始

### Windows

下载 [最新版本](https://github.com/SnishaperTeam/SniShaper/releases) 中的 `snishaper-windows-amd64.7z`（便携版）或 MSIX 安装包，解压 / 安装后运行 `snishaper.exe`。程序会自动请求管理员权限（TUN 模式需要），如提权失败则 TUN 功能不可用但其他功能正常。

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/zh-cn%20dark.svg" width="200"/>
</a>

### Linux

从 [最新版本](https://github.com/SnishaperTeam/SniShaper/releases) 下载 `snishaper-linux-amd64.tar.gz`，解压后运行：

```bash
tar -xzf snishaper-linux-amd64.tar.gz
sudo ./SniShaper
```

程序会自动申请 root 权限（TUN 模式需要），如提权失败则 TUN 功能不可用但代理等其他功能正常。当前提供 **amd64** 构建，基于 **GTK4 + WebKitGTK 6.0**（亦支持 GTK3）。

### CLI 版（无界面）

不需要图形界面、或在服务器 / SSH 环境中使用？本仓库内置 **SniShaper CLI**（`cli/` 目录）：

- **三平台支持**：Windows / Linux / macOS（amd64 + arm64）。
- **TUI 界面**：上屏实时滚动代理日志，下屏输入命令（支持中文别名），日志刷新再快也不会淹没输入。
- **后台模式**：`snishaper start` 常驻运行，`status` / `stop` / `logs` / `proxy` / `sysproxy` / `tun` / `config` / `ca` 子命令远程管理。
- **完整核心**：与 GUI 版共享同一套代理引擎（ECH 注入、TLS 分片、QUIC、TUN/gvisor、GFWList 分流、DoH、CF IP 池、NAT64、进化模式）。
- **移除更新检测**：无自动更新，适合长期运行的服务器场景。
- **版本一致**：与 GUI 版共用 `Package.appxmanifest` 作为唯一版本源。

构建方式详见 **[build.md — 构建产物矩阵](build.md#构建产物矩阵12-个目标)**，产物按 `build/bin/cli/<Platform>/<Arch>/` 组织，直接运行即可进入 TUI。

> **Darwin / macOS CLI 注意：** 当前 Darwin CLI 由于缺少可用于持续实机测试的 macOS 测试设备，实际使用中可能存在我们尚未预见的问题。若你在 Darwin / macOS 上发现任何异常，请及时提交 [Issue](https://github.com/SnishaperTeam/SniShaper/issues) 或 [Pull Request](https://github.com/SnishaperTeam/SniShaper/pulls)，帮助我们尽快定位和修复问题。

### 证书重新安装

在主界面点击「证书管理」-> 「**重置根证书**」。CLI 版使用 `snishaper ca regenerate` 后重新 `ca install`。

### 配置与启动

软件内置了丰富的官方规则，你也可以在「规则面板」中根据实际情况自定义规则，最后点击「**启动代理**」即可。

---

## 文档

想要了解更详细的技术原理、部署教程和自定义指南，请参阅 [**GitHub Wiki**](https://github.com/SnishaperTeam/SniShaper/wiki)：

- **[核心模式介绍](https://github.com/SnishaperTeam/SniShaper/wiki/Core-Proxy-Modes)**：了解 TLS-RF、QUIC 与 Server 模式的运行原理。
- **[规则自定义指南](https://github.com/SnishaperTeam/SniShaper/wiki/Custom-Rules-Guide)**：了解如何开发针对性的规则。
- **[界面配置实操](https://github.com/SnishaperTeam/SniShaper/wiki/GUI-Configuration)**：了解在 GUI 快速配置规则。
- **[常见问题排除](https://github.com/SnishaperTeam/SniShaper/wiki/FAQ)**：解决证书警告、规则不生效等常见问题。

---

## 构建与开发

本项目基于 **Wails v3 + React 19 + MUI** 构建，后端使用 **Go**，支持 Windows / Linux 双平台 GUI 与跨平台 CLI 共 12 个构建目标。完整的构建指南已拆分至 **[build.md](build.md)**，涵盖：

- **[构建产物矩阵](build.md#构建产物矩阵12-个目标)**：12 个目标的平台 / 架构 / 产物路径，构建脚本全部参数与示例。
- **[Windows 构建](build.md#windows-构建)**：`build_windows.ps1` 用法、MSIX 打包与行为说明。
- **[Linux 构建](build.md#linux-构建)**：GTK4 / GTK3 依赖安装与 `build.sh` 命令。
- **[版本与发布渠道](build.md#版本与发布渠道)** / **[开发环境](build.md#开发环境)**：版本源与工具链要求。
- **[持续集成](build.md#持续集成)**：双平台 CI 与发布流水线。
- **[跨平台说明](build.md#跨平台说明)**：Go build tags 与 CLI 子目录。

---

## 工具

### IP 扫描器 (tools/scanner.py)

通用 IP 扫描工具，用于扫描目标域名的可用代理 IP。

**用法：**

```bash
python tools/scanner.py <域名:端口> <IP段CIDR> [最大线程数]
```

**参数说明：**

| 参数 | 说明 | 示例 |
|------|------|------|
| 域名:端口 | 扫描目标 | `open.spotify.com:443` |
| IP段CIDR | 要扫描的 IP 段 | `35.186.224.0/24` |
| 最大线程数 | 并发线程数（默认64） | `128` |

**示例：**

```bash
# 扫描 Spotify 可用 IP
python tools/scanner.py open.spotify.com:443 35.186.224.0/24 128

# 扫描 Google 可用 IP
python tools/scanner.py google.com:443 34.0.0.0/8 256

# 扫描任意域名
python tools/scanner.py example.com:80 1.0.0.0/16 64
```

**输出：**

- 结果按响应速度从快到慢排列
- 日志保存在 `tools/logs/` 目录
- 有效 IP 保存为 `tools/logs/scan_*_valid.txt`

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

---

## 许可

[GNU Affero General Public License v3.0](LICENSE)（AGPL-3.0）。
