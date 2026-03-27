# SniShaper

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue?style=flat-square)]()

通过透传、本地中间人重写、TLS 分片、Warp 分流和反代中转绕过复杂网络环境阻断的代理工具，支持域前置和 ECH。

## 特性

- **五模式代理**
  - `transparent`：透明透传（自定义 host，仅DNS污染时）
  - `mitm`：中间人模式（本地 CA 解密，修改 sni 绕过阻断，针对可域前置/ECH网站）
  - `tls-rf`：TLS 分片模式（在 ClientHello 阶段做分片发送，）
  - `server`：服务端模式（连接上游动态反代服务器，无特征中转）
  - `warp`：WARP 分流（使用usque作为warp隧道，可选分流）

- **Warp 分流**
  - 完全使用了 https://github.com/Diniboy1123/usque 作为本地WARP子进程，启用后会启动本地 `usque` SOCKS5 隧道
  - 规则 `upstream` 设为 `warp` 时，该站点流量可按需经 Cloudflare Warp 转发
  - 默认不开启，需要时手动开启
  - 可解决少量网站如ChatGPT的连接问题
  - 本身不兼容ECH和Cloudflare IP优选。当然，可以自行优选出适合的Endpoint ip，填入配置即可。
- **ECH**
  - **动态 ECH**：通过内置 DoH 处理，动态获取 ECH 配置，针对支持的网站可配置开启ECH
  - **动态优选 IP 池**：智能优选边缘节点，提升访问性能

## 工作原理

```
浏览器 → SniShaper(127.0.0.1:port) → 规则匹配 → [模式选择: transparent/mitm/tls-rf/server 或 Warp 分流] → 上游握手 (ECH/Domain Fronting/TLS Fragment/Warp) → 目标直连
```

## 快速开始

### 1. 启动
运行 `snishaper.exe`。默认监听端口为 `127.0.0.1:8080`（可在设置中修改）。

## 构建

直接执行：

```powershell
wails build
```

发布流程会在 GitHub Actions 中自动把仓库内的 `rules/config.json` 与构建产物一起打包。
### 2. 安装证书（MITM 模式必需）
点击界面「证书管理」按钮，点击一键按照，自动安装生成的根证书到「受信任的根证书颁发机构」。

### 3. 配置加速
在ECH或规则页面输入想要加速的域名，根据实际情况生成配置。

### 4. 启用代理
点击主界面的「启动代理」并开启「系统代理」即可。


## 配置文件字段说明

| 字段 | 说明 |
|------|------|
| `domains` | 域名匹配列表 |
| `website` | 网站分组名（用于 UI 聚合展示） |
| `mode` | `transparent`、`mitm`、`tls-rf` 或 `server` |
| `upstream` | 上游地址（`IP:443`），或特殊值 `warp` |
| `sni_policy` | SNI 处理策略 |
| `ech_enabled` | 是否开启 ECH  |
| `use_cf_pool` | 是否启用优选 IP 池平衡负载与稳定性 |
| `cloudflare_config.warp_enabled` | 是否启用 Warp 功能 |
| `cloudflare_config.warp_endpoint` | Warp MASQUE 对端地址 |

## TLS 分片说明

`tls-rf` 模式不会像 MITM 那样终止客户端 TLS，也不会像透明模式那样完全原样透传。它会在转发到上游时对 TLS ClientHello 做分片发送，一定程度上规避对 SNI 识别。后向安全性不足。可能需要持续更新。借鉴自moi-si/lumine，感谢原作者的探索。

适用场景：
- 不希望安装本地根证书
- 目标站点对域前置很敏感


## 服务端部署

SniShaper的Server模式是可选的，用于绕过基于IP的封锁。它不是典型代理，而是修改原连接的urlpath并与动态反代服务器进行连接。
SniShaper 支持两种服务端部署方式：

### 方式一：Cloudflare Worker

```
客户端 → Worker → 目标网站
```

**部署步骤：**

1. 登录 [Cloudflare Dashboard](https://dash.cloudflare.com/)
2. 创建一个新的 Worker（hello world 模板）
3. 将 `sni-server/worker.js` 的内容复制到 Worker 编辑器
4. 在 Worker 设置中添加环境变量 `AUTH_SECRET`，设置密码
5. 部署 Worker，获取 Worker 域名（如 `xxx.workers.dev`）
6. 在客户端 Server 节点设置中填写域名和鉴权密码

### 方式二：VPS 部署（sni-server）

```
客户端 → Cloudflare Tunnel → VPS (sni-server) → 目标网站
```

**部署步骤：**

1. 准备一台 VPS（任何支持 Go 的 Linux 服务器）
2. 运行一键部署脚本：

```bash
curl -fsSL https://raw.githubusercontent.com/coolapijust/Shaper-Next/main/server/install.sh -o /tmp/sni-server-install.sh
sudo bash /tmp/sni-server-install.sh
```

3. 配置域名解析。推荐使用 Cloudflare Tunnel：
```bash
bash <(curl -sSL https://github.com/sky22333/shell/raw/main/dev/cf-tunnel.sh)
```

4. 在客户端配置中填写 Tunnel给的域名和鉴权密码

当然，也可以不用CDN，直接裸连VPS。后续版本会支持。

如果不想自行部署服务，可以直接使用集成的WARP。把Server规则的工作模式切换到warp即可。

## 常见问题

- **证书错误**：请确认证书已导入「受信任的根证书」分类，并务必重启浏览器
- **访问速度慢**：建议在「优选 IP 池」中添加更多当前环境下延迟较低的 Cloudflare 任播 IP；对具体网站规则进行修改，选出更好IP；换一个规则的工作模式

## 规则开发
可以根据本软件- [SniViewer](https://github.com/coolapijust/SniViewer)判断目标网站情况，根据测试结果针对性生成规则。
对于默认规则未覆盖的小站，成功率很高。
遇到改规则也连不上的网站，可直接填入Server/WARP规则
## 致谢

本项目在开发过程中参考并受益于以下优秀开源项目：

- [SNIBypassGUI](https://github.com/coolapijust/SniViewer)(https://github.com/racpast/SNIBypassGUI)
- [DoH-ECH-Demo](https://github.com/0xCaner/DoH-ECH-Demo)
- [lumine](https://github.com/moi-si/lumine)
- [usque](https://github.com/Diniboy1123/usque)

## 许可

[MIT License](LICENSE)
