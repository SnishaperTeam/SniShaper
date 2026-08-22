# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat-square)](LICENSE)
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/SniShaper/SniShaper/wiki)
[![GitHub Release](https://img.shields.io/github/v/release/SniShaper/SniShaper?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub Downloads](https://img.shields.io/github/downloads/SniShaper/SniShaper/total?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&logo=git)](https://github.com/SniShaper/SniShaper/commits/main)
[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SniShaper/SniShaper/build.yml?style=flat-square&logo=githubactions&label=CI)](https://github.com/SniShaper/SniShaper/actions)

**SniShaper** 是一款专为复杂网络环境设计的本地代理软件，通过 **ECH 注入**、**TLS 分片**、**QUIC 转换**、**会话迁移** 等多种协议栈技术，配合 **TUN 虚拟网卡** 接管全局流量，在复杂网络环境下提供稳定灵活的访问体验。

本项目为 **Windows 与 Linux 双平台** 仓库，共用同一套代码与版本机制，平台相关逻辑通过 Go build tags 隔离。

> 需要无图形界面的终端版本？本仓库内置 **SniShaper CLI**（`cli/` 目录）——跨平台（Windows / Linux / macOS）headless 版，内置 TUI 分屏界面（实时日志 + 命令面板），保留全部核心代理能力。构建与用法见 [CLI 说明](cli/README.md)。

---

## 特性

- **多模式代理**：MITM（中间人）、Transparent（透明）、TLS-RF（TLS 分片）、QUIC、Migration（会话迁移）、Direct（直连）等多种模式覆盖不同场景。
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

下载 [最新版本](https://github.com/SniShaper/SniShaper/releases) 中的 `snishaper-windows-amd64.7z`（便携版）或 MSIX 安装包，解压 / 安装后运行 `snishaper.exe`。程序会自动请求管理员权限（TUN 模式需要），如提权失败则 TUN 功能不可用但其他功能正常。

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/zh-cn%20dark.svg" width="200"/>
</a>

### Linux

从 [最新版本](https://github.com/SniShaper/SniShaper/releases) 下载 `snishaper-linux-amd64.tar.gz`，解压后运行：

```bash
tar -xzf snishaper-linux-amd64.tar.gz
sudo ./SniShaper
```

程序会自动申请 root 权限（TUN 模式需要），如提权失败则 TUN 功能不可用但代理等其他功能正常。当前提供 **amd64** 构建，基于 **GTK4 + WebKitGTK 6.0**（亦支持 GTK3）。

### CLI 版（无界面）

不需要图形界面、或在服务器 / SSH 环境中使用？本仓库内置 **SniShaper CLI**（`cli/` 目录，详见 [CLI 说明](cli/README.md)）：

- **三平台支持**：Windows / Linux / macOS（amd64 + arm64）。
- **TUI 界面**：上屏实时滚动代理日志，下屏输入命令（支持中文别名），日志刷新再快也不会淹没输入。
- **后台模式**：`snishaper start` 常驻运行，`status` / `stop` / `logs` / `proxy` / `sysproxy` / `tun` / `config` / `ca` 子命令远程管理。
- **完整核心**：与 GUI 版共享同一套代理引擎（ECH 注入、TLS 分片、QUIC、TUN/gvisor、GFWList 分流、DoH、CF IP 池、NAT64、进化模式）。
- **移除更新检测**：无自动更新，适合长期运行的服务器场景。
- **版本一致**：与 GUI 版共用 `Package.appxmanifest` 作为唯一版本源。

```bash
# 仓库内构建 CLI（全平台：windows/linux/darwin x amd64/arm64）
./build.sh --cli          # Unix / macOS
.\build_windows.ps1 -Build backend -Cli -Silent   # Windows

# 产物在 build/bin/cli/，直接运行 TUI
./build/bin/cli/snishaper-cli-linux-amd64
```

### 证书重新安装

在主界面点击「证书管理」-> 「**重置根证书**」。CLI 版使用 `snishaper ca regenerate` 后重新 `ca install`。

### 配置与启动

软件内置了丰富的官方规则，你也可以在「规则面板」中根据实际情况自定义规则，最后点击「**启动代理**」即可。

---

## 文档

想要了解更详细的技术原理、部署教程和自定义指南，请参阅 [**GitHub Wiki**](https://github.com/SniShaper/SniShaper/wiki)：

- **[核心模式介绍](https://github.com/SniShaper/SniShaper/wiki/Core-Proxy-Modes)**：了解 TLS-RF、QUIC 与 Server 模式的运行原理。
- **[规则自定义指南](https://github.com/SniShaper/SniShaper/wiki/Custom-Rules-Guide)**：了解如何开发针对性的规则。
- **[界面配置实操](https://github.com/SniShaper/SniShaper/wiki/GUI-Configuration)**：了解在 GUI 快速配置规则。
- **[常见问题排除](https://github.com/SniShaper/SniShaper/wiki/FAQ)**：解决证书警告、规则不生效等常见问题。

---

## 构建与开发

本项目基于 **Wails v3 + React 19 + MUI** 构建，后端使用 **Go**。同一仓库同时产出 Windows 与 Linux 可执行文件。

### Windows 构建

```powershell
# 克隆仓库
git clone https://github.com/SniShaper/SniShaper.git
cd SniShaper

# 完整编译（交互模式，自动安装依赖、可选 MSIX 打包）
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# 或使用 PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1
```

#### 构建脚本命令行参数

`build_windows.ps1` 支持以下参数，可跳过交互式选择：

| 参数 | 可选值 | 说明 |
| -------------- | ------------------------------ | ------------------------------------------------------------ |
| `-Build` | `frontend` / `backend` / `all` | 指定构建目标 |
| `-Lang` | `en` / `cn` / `ru` | 指定界面语言 |
| `-InstallDeps` | 无值（开关） | 安装前端 npm 依赖 |
| `-BuildMsix` | 无值（开关） | 构建 MSIX 安装包 |
| `-SkipSign` | 无值（开关） | 跳过 MSIX 签名，生成的文件添加 `unsigned_` 前缀（需配合 `-BuildMsix`） |
| `-Silent` | 无值（开关） | 静默模式，跳过所有交互提示 |

**用法示例：**

```powershell
# 仅构建前端（中文界面）
.\build_windows.ps1 -Build frontend -Lang cn

# 仅构建后端（英文界面）
.\build_windows.ps1 -Build backend -Lang en

# 同时构建前后端，并安装依赖
.\build_windows.ps1 -Build all -Lang cn -InstallDeps

# 构建前后端并生成 MSIX 安装包（默认签名）
.\build_windows.ps1 -Build all -BuildMsix

# 构建前后端并生成未签名的 MSIX（跳过签名）
.\build_windows.ps1 -Build all -BuildMsix -SkipSign

# 静默模式（CI/CD 适用，无交互）
.\build_windows.ps1 -Silent

# 静默模式构建并打包（跳过签名）
.\build_windows.ps1 -Build all -Silent -BuildMsix -SkipSign

# 无参数 = 交互模式（原有行为）
.\build_windows.ps1
```

### Linux 构建

Linux 构建使用 `build_linux.sh`，在 Linux 本机（或 Windows 上的 WSL2）执行。

#### 依赖（Ubuntu / Debian）

```bash
# GTK4 + WebKitGTK 6.0（默认）
sudo apt-get update
sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev

# 或使用 GTK3 + webkit2gtk-4.1
# sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

#### 构建命令

```bash
# 克隆仓库
git clone https://github.com/SniShaper/SniShaper.git
cd SniShaper

# 仅编译后端（使用已有的 frontend/dist）
./build_linux.sh

# 先构建前端再编译后端
./build_linux.sh --with-frontend

# 使用 GTK3 + webkit2gtk-4.1
./build_linux.sh --gtk3
```

构建产物输出到 `build/bin/SniShaper`（含 `rules/`、`config/` 种子文件）。TUN / 系统代理需要 root，运行时 `sudo ./build/bin/SniShaper`。

### 版本与发布渠道

版本号与发布渠道（`release` / `beta` / `alpha` / `rc`）由项目根目录的 `Package.appxmanifest` **统一提供**：

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

Windows 与 Linux 构建均从此文件读取版本信息，并通过 ldflags 注入（`snishaper/app.buildVersion`、`snishaper/app.buildChannel`）。仓库中不存在独立的版本 JSON 文件。

### 开发环境

- `Go 1.25+`
- `Node.js 24+` / `npm 11+`
- Windows：MSVC 工具链（Wails v3）、WinApp CLI（MSIX 打包）
- Linux：GTK4 / WebKitGTK 或 GTK3 开发包（见上）
- TUN 模式依赖 gvisor 网络栈（Windows 通过 `with_gvisor` 构建 tag 启用）

构建产物：

- 前端资源位于 `frontend/dist`
- Windows 可执行文件位于 `build/bin/snishaper.exe`
- Linux 可执行文件位于 `build/bin/SniShaper`

---

## 持续集成

双平台 CI 流水线：

- **`build.yml`**：每次 push / PR 触发，在 `windows-2025` 上构建 Windows、在 `ubuntu-24.04` 上构建 Linux，并执行编译与二进制冒烟验证。
- **`_release_pipeline.yml`**：发布流水线。Windows runner 产出 MSIX 与 `snishaper-windows-amd64.7z` 便携包，Ubuntu runner 产出 `snishaper-linux-amd64.tar.gz`，最后在 Windows runner 合并两个平台的产物并创建 GitHub Release。Release notes 优先由 runner 本地 Ollama（默认 `qwen3.5:2b`）生成摘要；Ollama 不可用时降级为分类 commit 列表。

---

## 跨平台说明

Windows 与 Linux 由同一仓库构建，平台相关实现通过 Go build tags 隔离（如 `//go:build linux` / `windows`）。无需再访问独立的 Linux 仓库。

CLI（headless）版本作为本仓库的 `cli/` 子目录维护，与 GUI 共用同一套核心代码与版本机制（`Package.appxmanifest`），构建方式见 [CLI 说明](cli/README.md)。

## 致谢

本项目受益于以下优秀开源项目的启发：

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)

## 贡献者

感谢以下贡献者对本仓库的贡献：

| <a href="https://github.com/mechrevo"><img src="https://avatars.githubusercontent.com/mechrevo" width="40" height="40" style="border-radius: 50%;" alt="mechrevo" /></a> | <a href="https://github.com/dongzheyu"><img src="https://avatars.githubusercontent.com/dongzheyu" width="40" height="40" style="border-radius: 50%;" alt="dongzheyu" /></a> | <a href="https://github.com/JetCPP-dongle"><img src="https://avatars.githubusercontent.com/JetCPP-dongle" width="40" height="40" style="border-radius: 50%;" alt="JetCPP-dongle" /></a> |
| :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
| [mechrevo](https://github.com/mechrevo) | [dongzheyu](https://github.com/dongzheyu) | [JetCPP-dongle](https://github.com/JetCPP-dongle) |
| <a href="https://github.com/lzpls"><img src="https://avatars.githubusercontent.com/lzpls" width="40" height="40" style="border-radius: 50%;" alt="lzpls" /></a> |
| [lzpls](https://github.com/lzpls) |

## 星标历史

## Star History

<a href="https://www.star-history.com/?repos=snishaper%2Fsnishaper&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&theme=dark&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left&sealed_token=8Q__19KTE6g7OqVIseB0o2elHwSh9GjE93LPnbu5UWeQ-0vS0Qpt7BzQIUgKqNYIObs96Y6oFUbTB98qvun_ivkhW1TG1AEr701tG403fsGTcLcbLITh7Q" />
 </picture>
</a>

---

## 项目活跃度与贡献者

### 活跃度徽章

[![GitHub contributors](https://img.shields.io/github/contributors/SniShaper/SniShaper?style=flat-square&label=总贡献者)](https://github.com/SniShaper/SniShaper/graphs/contributors)
[![GitHub commit activity](https://img.shields.io/github/commit-activity/m/SniShaper/SniShaper?style=flat-square&label=月均提交)](https://github.com/SniShaper/SniShaper/graphs/contributors)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&label=最近提交)](https://github.com/SniShaper/SniShaper/commits/main)

### 综合活跃度趋势

<div align="center">
<a href="https://repobeats.axiom.co/" target="_blank">
<img src="https://repobeats.axiom.co/api/embed/f62c98a5231da45588ee71f26e3c1cc3f64edb6b.svg" alt="Repobeats analytics" />
</a>
</div>

### 核心贡献者

<div align="center">
<a href="https://github.com/SniShaper/SniShaper/graphs/contributors" target="_blank">
<img src="https://contrib.rocks/image?repo=SniShaper/SniShaper" alt="Contributors" />
</a>
</div>

---

## 许可

[GNU Affero General Public License v3.0](LICENSE)（AGPL-3.0）。
