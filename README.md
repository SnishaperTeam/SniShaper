# SniShaper

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()
[![Wiki](https://img.shields.io/badge/Docs-Wiki-orange?style=flat-square)](https://github.com/coolapijust/snishaper/wiki)

**SniShaper** 是一款专为复杂网络环境设计的本地代理工具。它集成了多种技术，包括 **ECH 注入**、**TLS-RF 分片**、**QUIC 重建连接** 以及 **Server 模式轻量中转**，旨在为用户提供稳定的访问体验。

---

## 特性

- **六模式全方位覆盖**：支持从轻量级的 `transparent` 到高级的 `server` 转发，满足不同需求。
- **灵活策略**：
  - **TLS-RF (TLS 分片)**：通过分片规避针对 SNI 的精准阻断。
  - **QUIC 重建**：利用 quic-go 的混淆特性绕过常规 SNI 检测。
  - **ECH 注入**：自动获取并注入 echconfig。
- **优选 IP 与 WARP**：集成 Cloudflare 优选 IP 池与 WARP Masque 隧道。

---

## 快速开始

### 1. 运行
下载 [最新版本](https://github.com/coolapijust/snishaper/releases) 并运行 `snishaper.exe`。

### 2. 证书重新安装
在主界面点击「证书管理」-> 「**点击重新安装证书**」。

### 3. 配置与启动
软件内置了丰富的官方规则，你也可以在「规则面板」中根据实际情况自定义规则，最后点击「**启动代理**」即可。

---

## 文档 

想要了解更详细的技术原理、部署教程和自定义指南，请参阅 [**GitHub Wiki**](https://github.com/coolapijust/snishaper/wiki)：

-  **[核心模式介绍](https://github.com/coolapijust/snishaper/wiki/Core-Proxy-Modes)**：了解 TLS-RF、QUIC 与 Server 模式的运行原理。
-  **[规则自定义指南](https://github.com/coolapijust/snishaper/wiki/Custom-Rules-Guide)**：了解如何开发针对性的规则。
-  **[界面配置实操](https://github.com/coolapijust/snishaper/wiki/GUI-Configuration)**：了解在GUI快速配置规则。
-  **[服务端部署](https://github.com/coolapijust/snishaper/wiki/Server-Deployment)**：在 CF Workers 或 VPS 上架设你自己的 Server 节点。
-  **[常见问题排除](https://github.com/coolapijust/snishaper/wiki/FAQ)**：解决证书警告、规则不生效等常见问题。

---

## 构建与开发

本项目基于 **Wails v3** 构建。

```powershell
# 克隆仓库
git clone https://github.com/coolapijust/snishaper.git
cd snishaper

# 构建前端
cd frontend && npm run build && cd ..
# 构建后端
go build -ldflags="-H windowsgui" -o build/bin/snishaper.exe .
```

---

## 致谢

本项目受益于以下优秀开源项目的启发：

- [SNIBypassGUI](https://github.com/coolapijust/SniViewer)
- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)
- [usque](https://github.com/Diniboy1123/usque)

## 许可

[MIT License](LICENSE)
