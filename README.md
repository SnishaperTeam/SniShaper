# SniShaper

[中文](README.md) | [English](README_EN.md) | [Русский](README_RU.md)

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/SniShaper/SniShaper/wiki)
[![GitHub Release](https://img.shields.io/github/v/release/SniShaper/SniShaper?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub Downloads](https://img.shields.io/github/downloads/SniShaper/SniShaper/total?style=flat-square&logo=github)](https://github.com/SniShaper/SniShaper/releases)
[![GitHub last commit](https://img.shields.io/github/last-commit/SniShaper/SniShaper?style=flat-square&logo=git)](https://github.com/SniShaper/SniShaper/commits/main)
[![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/SniShaper/SniShaper/build.yml?style=flat-square&logo=githubactions&label=CI)](https://github.com/SniShaper/SniShaper/actions)

**SniShaper** 是一款专为复杂网络环境设计的本地代理工具，集成 **ECH 注入**、**TLS-RF 分片**、**QUIC 混淆**、**会话迁移** 等多种协议栈，配合 **TUN 虚拟网卡** 实现全流量接管，提供稳定、灵活的科学上网体验。

---

## 特性

- **多模式代理**：MITM（中间人）、Transparent（透明）、TLS-RF（TLS 分片）、QUIC、Migration（会话迁移）、Direct（直连）等多种模式覆盖不同场景。
- **TUN 虚拟网卡**：原生 TUN 支持，全局流量劫持，自动路由与 DNS 劫持。
- **ECH 注入**：自动获取并注入 ECH Config，支持 DoH 发现与热更新。
- **智能分流**：基于 GFWList 自动识别被屏蔽域名，自动路由引擎无需手动配置即可分流。
- **DoH 加密 DNS**：内置抗污染 DNS 解析器，支持多节点故障转移。
- **Cloudflare IP 优选池**：自动测速、健康检查与刷新。
- **SOCKS5 代理**：内置 SOCKS5 服务端，支持独立端口。
- **NAT64 转换**：在纯 IPv6 网络下自动转换 IPv4 地址。
---

## 快速开始

### 1. 运行
下载 [最新版本](https://github.com/SniShaper/SniShaper/releases) 并运行 `snishaper.exe`。程序会自动申请管理员权限（TUN 模式需要），如提权失败则 TUN 功能不可用但其他功能正常。

<a href="https://apps.microsoft.com/detail/9n11mrrsfs8n" target="_self">
<img src="https://get.microsoft.com/images/zh-cn%20dark.svg" width="200"/>
</a>

### 2. 证书重新安装
在主界面点击「证书管理」-> 「**点击重新安装证书**」。

### 3. 配置与启动
软件内置了丰富的官方规则，你也可以在「规则面板」中根据实际情况自定义规则，最后点击「**启动代理**」即可。

---

## 文档 

想要了解更详细的技术原理、部署教程和自定义指南，请参阅 [**GitHub Wiki**](https://github.com/SniShaper/SniShaper/wiki)：

-  **[核心模式介绍](https://github.com/SniShaper/SniShaper/wiki/Core-Proxy-Modes)**：了解 TLS-RF、QUIC 与 Server 模式的运行原理。
-  **[规则自定义指南](https://github.com/SniShaper/SniShaper/wiki/Custom-Rules-Guide)**：了解如何开发针对性的规则。
-  **[界面配置实操](https://github.com/SniShaper/SniShaper/wiki/GUI-Configuration)**：了解在GUI快速配置规则。
-  **[服务端部署](https://github.com/SniShaper/SniShaper/wiki/Server-Deployment)**：在 CF Workers 或 VPS 上架设你自己的 Server 节点。
-  **[常见问题排除](https://github.com/SniShaper/SniShaper/wiki/FAQ)**：解决证书警告、规则不生效等常见问题。

---

## 构建与开发

本项目基于 **Wails v3** 构建。

```powershell
# 克隆仓库
git clone https://github.com/SniShaper/snishaper.git
cd snishaper

# 安装前端依赖
cd frontend
npm install

# 构建前端静态资源
npm run build
cd ..

# 一次性完成完整编译（交互模式）
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# 或使用 PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1

# Go主程序编译（脚本会自动执行 go mod download）
go build -tags with_gvisor -ldflags="-s -w" -o "build/bin/snishaper.exe"
```

### 构建脚本命令行参数

`build_windows.ps1` 支持以下参数，可跳过交互式选择：

| 参数 | 可选值 | 说明 |
|------|--------|------|
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

开发环境建议：

- `Go 1.25+`
- `Node.js 24+`
- `npm 11+`
- `gVisor`（TUN 模式需要，Linux 需安装 `gvisor` 包）

构建产物：

- 前端资源位于 `frontend/dist`
- 可执行文件位于 `build/bin/snishaper.exe`

---
## 跨平台
本程序支持 Windows和Linux 平台，Linux版请参见[Linux版](https://github.com/dongzheyu/SniShaper-Linux/)
。
## 致谢

本项目受益于以下优秀开源项目的启发：

- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)
- [usque](https://github.com/Diniboy1123/usque)

感谢以下贡献者对本仓库的贡献：

| <a href="https://github.com/mechrevo"><img src="https://avatars.githubusercontent.com/mechrevo" width="40" height="40" style="border-radius: 50%;" alt="mechrevo" /></a> | <a href="https://github.com/dongzheyu"><img src="https://avatars.githubusercontent.com/dongzheyu" width="40" height="40" style="border-radius: 50%;" alt="dongzheyu" /></a> | <a href="https://github.com/JetCPP-dongle"><img src="https://avatars.githubusercontent.com/JetCPP-dongle" width="40" height="40" style="border-radius: 50%;" alt="JetCPP-dongle" /></a> |
| :---: | :---: | :---: |
| [mechrevo](https://github.com/mechrevo) | [dongzheyu](https://github.com/dongzheyu) | [JetCPP-dongle](https://github.com/JetCPP-dongle) |
## 星标历史

<a href="https://www.star-history.com/?repos=snishaper%2Fsnishaper&type=date&legend=top-left">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&theme=dark&legend=top-left" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left" />
   <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=snishaper/snishaper&type=date&legend=top-left" />
 </picture>
</a>

---
## 许可

[MIT License](LICENSE)
