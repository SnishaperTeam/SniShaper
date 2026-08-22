# SniShaper CLI

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue?style=flat-square)](LICENSE)

**SniShaper CLI** 是 [SniShaper](https://github.com/SniShaper/SniShaper) 的无界面（headless）版本：剥离 Wails + React 图形界面，保留全部核心代理能力，提供 **Windows / Linux / macOS** 三平台支持，并移除了自动更新检测。

主交互方式为 **TUI**（终端分屏界面）：上半屏实时滚动代理日志，下半屏输入命令控制服务——日志刷新再快也不会淹没你的输入。

核心能力与原版一致：**ECH 注入**、**TLS 分片（TLS-RF）**、**QUIC 连接重建**、**会话迁移**、**TUN 虚拟网卡（gvisor）**、**GFWList 智能分流**、**抗污染 DoH**、**Cloudflare IP 优选池**、**NAT64**、**进化模式（Evolution）** 规则测试。

---

## 特性

- **多模式代理**：MITM（中间人）、Transparent（透明）、TLS-RF（TLS 分片）、QUIC、Migration（会话迁移）、Direct（直连）。
- **TUN 虚拟网卡**：Windows 走 WinTun、Linux/macOS 走 gvisor 网络栈，全局流量透明劫持，自动路由与 DNS 劫持。
- **ECH 注入**：自动获取并注入 ECH Config，支持 DoH 发现与热更新。
- **智能分流**：基于 GFWList 自动识别被屏蔽域名，自动路由引擎无需手动配置。
- **加密 DNS**：内置抗污染 DNS 解析器，支持多节点故障转移。
- **Cloudflare IP 优选池**：自动测速、健康检查与刷新。
- **NAT64 支持**：更灵活的 IP 出口和服务访问。
- **进化模式（Evolution）**：自动测试多种规则组合，寻找目标站点最优访问方式。

---

## 快速开始

### 启动 TUI

```bash
# Windows (PowerShell / Windows Terminal)
.\snishaper.exe

# Linux / macOS
./snishaper
```

TUI 分为三个区域：

- **顶部状态栏**：代理 / 系统代理 / TUN 开关状态、HTTP 端口、当前模式，每秒刷新。
- **中部日志面板**：合并展示应用层与核心进程日志，自动滚动跟随；用**鼠标滚轮 / PageUp / PageDown** 翻阅历史日志（翻阅时暂停自动跟随，按 **End** 或滚轮回到底部恢复），**Tab** 切换焦点。
- **底部命令输入**：输入命令回车执行，支持中文命令别名：

| 命令 | 说明 |
| -------- | ------------------------------------------ |
| `start` / `启动` / `proxy on` | 启动代理（HTTP + SOCKS5） |
| `stop` / `停止` / `proxy off` | 停止代理 |
| `sysproxy on\|off` / `系统代理` | 开启/关闭系统代理 |
| `tun on\|off` | 切换 TUN 模式（需要管理员/root） |
| `status` / `状态` | 查看当前状态 |
| `clear` / `清屏` | 清空日志面板 |
| `quit` / `退出` | 停止服务并退出 |
| `help` / `帮助` | 显示命令帮助 |

`Ctrl+C` 干净退出（自动停止 TUN、关闭受管的系统代理、关闭核心子进程）。

### 命令行子命令

| 命令 | 说明 |
| -------- | -------------------------------------------- |
| `snishaper` | 启动 TUI（默认） |
| `snishaper start` | 后台启动服务（`--serve` 常驻进程） |
| `snishaper stop` | 停止运行中的服务 |
| `snishaper status` | 查看服务/代理/TUN/系统代理状态 |
| `snishaper logs [N]` | 打印最近 N 行日志（默认 100） |
| `snishaper proxy on\|off` | 启动/停止代理 |
| `snishaper sysproxy on\|off` | 开启/关闭系统代理 |
| `snishaper tun on\|off` | 切换 TUN 模式（需要管理员/root） |
| `snishaper config get [key]` / `set <key> <value>` | 查看/修改配置 |
| `snishaper ca ...` | 根证书管理（见下） |
| `snishaper version` | 打印版本号 |
| `snishaper help` | 显示帮助 |

### 根证书管理

HTTPS 拦截（MITM / ECH）需要安装本程序生成的根证书：

```bash
snishaper ca status       # 查看安装状态
snishaper ca install      # 安装到系统信任库（Windows 为当前用户根存储；Linux/macOS 需管理员）
snishaper ca uninstall    # 卸载已安装的根证书
snishaper ca export       # 导出为 ca.crt
snishaper ca path         # 显示证书文件路径
snishaper ca regenerate   # 重新生成根证书（之后需重新安装）
```

### 配置与规则

配置文件与规则文件随可执行文件分发，首次运行会复制到用户配置目录（`~/.config/snishaper`，见 `common.UserConfigDir()`）：

- `config/settings.json`：监听端口、SOCKS5、TUN、主题等设置。
- `rules/config.json`：站点组规则（约 3800 行）、MITM/ECH/SNI-fake 配置、GFWList。

修改方式：直接编辑文件，或 `snishaper config set <key> <value>`（运行中的服务会自动重载配置）。

---

## 构建与开发

CLI 版随主仓库统一构建（共享 `core/`、`proxy/`、`pkg/` 等代码），产物输出到 **`build/bin/cli/`**。

### 构建脚本

```powershell
# PowerShell（Windows，需 Go 1.25+）
.\build_windows.ps1 -Build backend -Cli -Silent

# 或仅指定平台 / 架构（-Cli 为开关，构建全平台；如需单平台请直接用 go build）
```

```bash
# bash（Linux / macOS / Git Bash）
./build.sh --cli          # 仅构建 CLI（全平台 6 目标）
./build.sh --all          # GUI + CLI 一起构建
```

构建产物（`build/bin/cli/`），随附 `config/`、`rules/` 种子文件：

```
build/bin/cli/
├── snishaper-cli-windows-amd64.exe
├── snishaper-cli-windows-arm64.exe
├── snishaper-cli-linux-amd64
├── snishaper-cli-linux-arm64
├── snishaper-cli-darwin-amd64
├── snishaper-cli-darwin-arm64
├── config/settings.json
└── rules/config.json
```

> `-tags with_gvisor` 必须保留：TUN 数据面依赖 gvisor 网络栈。交叉编译使用 `CGO_ENABLED=0`，无需 C 工具链。

### 开发环境

- `Go 1.25+`
- TUN 模式依赖 gvisor 网络栈（`with_gvisor` 构建 tag 启用）
- Windows 终端要求：tcell 不支持 mintty/ConEmu，请使用 **Windows Terminal** 或 conhost 运行 TUI（PowerShell 7 可直接使用）

### 版本机制

与 GUI 版完全一致：版本号单一来源为仓库根目录的 **`Package.appxmanifest`**：

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

读取/注入逻辑与 GUI 版相同，优先级从高到低：

1. **ldflags 注入**（构建脚本 / CI 发布流水线）：`-X snishaper/cli/app.buildVersion=<版本> -X snishaper/cli/app.buildChannel=<频道>`，发布版以注入值为准；
2. **运行时读取** `Package.appxmanifest`（可执行文件目录向上搜索）；
3. 兜底默认 `1.29`。

软件内查看版本：CLI `snishaper version`、TUI 命令面板输入 `version`。

---

## 持续集成与发布

- **`build.yml`**：每次 push / PR 触发，在 `windows-2025`、`ubuntu-24.04`、`macos-14` 上构建全部 6 个目标（windows/linux/darwin × amd64/arm64），执行 `version` / `help` 冒烟验证。
- **4 个发布频道**（与 GUI 版一致的发布流程）：
  - `release.yml`：正式版，`v*` tag push 或手动触发；
  - `beta.yml` / `alpha.yml` / `rc.yml`：预发布频道，手动触发（`version_suffix` 可指定，如 `beta.2`，缺省 `beta.1`）。
- **`_release_pipeline.yml`**：共享发布流水线——解析 `version.json` 与频道、六目标构建并打包含 `config/`、`rules/` 种子文件、生成 release notes、创建 GitHub Release。
- **Release notes**：优先由 runner 本地 **Ollama**（默认 `qwen3.5:2b`，可用 `OLLAMA_MODEL` secret 覆盖）生成英文摘要；Ollama 不可用时降级为 OpenAI 兼容 API（`LLM_API_KEY` / `LLM_MODEL` / `LLM_BASE_URL`），再不可用时降级为分类 commit 列表。变更基线取**上一个 stable（非 prerelease）release**；正式严谨英文、无 emoji、附每个资产文件的用途与 MD5。
- 发布资产：每目标一个便携压缩包（`snishaper-<os>-<arch>.zip|.tar.gz`，含二进制 + `config/` + `rules/`）。不生成 MSIX / Inno Setup 安装程序。

---

## 平台说明

- **Windows**：TUN 与系统代理修改需要管理员权限（提示提权失败时 TUN 不可用，其余功能正常）。HTTP 代理默认 `127.0.0.1:8080`，SOCKS5 默认 `8081`。
- **Linux**：系统代理通过 gsettings（GNOME）；TUN / 系统代理需要 root——代码会通过 `$SUDO_USER` 重派发 gsettings，确保修改落在桌面会话而非 root 的 dconf。
- **macOS**：系统代理通过 `networksetup`（需要管理员）；TUN 需要 root。
- 所有平台的运行日志落在 `<execDir>/log/`（`core_stdout.log` 为核心子进程原始输出）。

---

## 架构

```
main.go    CLI/TUI 入口：子命令、服务生命周期、后台启动（--serve）
tui.go     tview 分屏界面（状态栏 + 日志面板 + 命令输入）
app/       编排层（无 GUI 依赖；UIAdapter 可空；Startup/Shutdown 替代 wails 生命周期）
core/      核心子进程：RPC 服务端 + 代理运行时（--core 模式，127.0.0.1:18933）
proxy/     代理引擎：MITM、SOCKS5、TLS 分片、TUN 流量、自动路由
pkg/       证书管理、CF IP 池、DoH 解析、sing-tun、系统代理（三平台实现）
common/    路径与日志工具
config/    settings.json（运行时配置）
rules/     config.json（站点组规则）
```

进程模型与原版一致：主进程拉起 `--core` 子进程（独占代理运行时），控制走本地 RPC（`127.0.0.1:18933`），核心子进程的 stdout/stderr 重定向到日志文件，避免污染 TUI 终端。TUI / 服务进程监控 core 存活，core 退出时优雅关闭（含系统代理恢复）。

---

## 许可

[GNU Affero General Public License v3.0](LICENSE)（AGPL-3.0）。
