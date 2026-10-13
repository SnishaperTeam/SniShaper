# SniShaper 平台与快速开始

[中文](Platform.md) | [English](Platform_EN.md) | [Русский](Platform_RU.md)

> 返回 [README](README.md)

本项目提供跨平台支持：**Windows 与 Linux** 共用同一套代码与版本机制，平台相关逻辑通过 Go build tags 隔离。

## 目录

- [快速开始](#快速开始)
  - [Windows](#windows)
  - [Linux](#linux)
  - [CLI 版](#cli-版)
  - [证书重新安装](#证书重新安装)
  - [配置与启动](#配置与启动)
- [移动端](#移动端)

---

## 快速开始

### Windows

下载 [最新版本](https://github.com/SnishaperTeam/SniShaper/releases) 中的 `snishaper-windows-amd64.7z`（便携版）或 MSIX 安装包，解压 / 安装后运行 `snishaper.exe`。程序会自动请求管理员权限（TUN 模式需要），如拒绝则 TUN 功能不可用但其他功能正常。

### Linux

从 [最新版本](https://github.com/SnishaperTeam/SniShaper/releases) 下载 `snishaper-linux-amd64.tar.gz`，解压后运行：

```bash
tar -xzf snishaper-linux-amd64.tar.gz
sudo ./SniShaper
```

程序会自动申请 root 权限（TUN 模式需要），如提权失败则 TUN 功能不可用但代理等其他功能正常。当前提供 **amd64** 构建，基于 **GTK4 + WebKitGTK 6.0**（亦支持 GTK3）。

### CLI 版

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

## 移动端

需要移动端客户端？SniShaper 有两个基于相同路由理念的配套版本：

- **[Lumine for Android](https://github.com/Snishaper/lumine-for-android)**：Kotlin + Jetpack Compose（Material Design 3）原生界面，Go（enimul）核心经 gomobile 绑定为单个 AAR，无 WebView 内嵌；支持订阅管理、规则编辑、实时日志与后台保活，也可通过 F-Droid（`com.moi.lumine`）获取。
- **[Lumine for HarmonyOS](https://github.com/SnishaperTeam/lumine-for-harmonyos)**：ArkTS + ArkUI 原生界面（深浅色），便携 C++17 核心经 NAPI 接入为单个 `liblumine_napi.so`，无 WebView 内嵌；支持 VpnExtensionAbility（TUN）隧道、本地代理监听（SOCKS5 / HTTP 回环入站）、订阅管理、规则编辑、实时日志与运行状态通知。
