# SniShaper 构建指南

[中文](build.md) | [English](build_EN.md) | [Русский](build_RU.md)

> 返回 [README](README.md)

本项目基于 **Wails v3 + React 19 + MUI** 构建，后端使用 **Go**。`build.sh`（Linux / macOS / WSL）与 `build_windows.ps1`（Windows）共用同一套目标矩阵、参数语义与输出目录。

## 目录

- [构建产物矩阵（12 个目标）](#构建产物矩阵12-个目标)
- [Windows 构建](#windows-构建)
- [Linux 构建](#linux-构建)
- [版本与发布渠道](#版本与发布渠道)
- [开发环境](#开发环境)
- [持续集成](#持续集成)
- [跨平台说明](#跨平台说明)

---

## 构建产物矩阵（12 个目标）

| 类型 | 平台 | 架构 | 产物 |
| --- | --- | --- | --- |
| CLI | Windows | `x64` / `x86` / `arm64` | `build/bin/cli/Windows/<arch>/snishaper.exe` |
| CLI | Linux | `x64` / `arm64` | `build/bin/cli/Linux/<arch>/snishaper` |
| CLI | Darwin | `x64` / `arm64` | `build/bin/cli/Darwin/<arch>/snishaper` |
| GUI | Windows | `x64` / `x86` / `arm64` | `build/bin/gui/Windows/<arch>/snishaper.exe` |
| GUI | Linux | `x64` / `arm64` | `build/bin/gui/Linux/<arch>/SniShaper` |

- 7 个 CLI + 5 个 GUI = **12 个目标**；每个目标目录都附带 `config/` 与 `rules/` 种子文件。
- GUI 不构建 Darwin（Wails 桌面端仅支持 Windows / Linux）；`x86` 仅存在于 Windows。
- GUI 依赖 GTK/WebKit（Linux）与前端产物，**只在同平台的原生环境构建**；CLI 是纯 Go（`CGO_ENABLED=0`），可在一个运行器上交叉产出全部 7 个目标。

### 静默参数

| build.sh | build_windows.ps1 | 说明 |
| --- | --- | --- |
| `--platform` / `-p` | `-Platform`（数组：`windows,linux`） | 平台：`windows` / `linux` / `darwin` / `all` |
| `--arch` / `-a` | `-Arch`（数组：`x64,arm64`） | 架构：`x64` / `x86` / `arm64` / `all` |
| `--type` / `-t` | `-Type`（数组：`cli,gui`） | 类型：`cli` / `gui` / `all` |
| `--all` | `-All` | 全部平台 + 全部架构（`--type` / `-Type` 仍可收窄） |
| `--dry-run` | `-DryRun` | 只解析并打印构建计划，不执行构建 |
| `--ci` | `-CI` | CI 模式：禁止交互、禁止导出交叉 `CC`/`CXX` |
| `--cross` | `-Cross` | 本地开发：允许 Linux GUI 使用交叉工具链（`-CI` 下失效） |
| `--install-deps` | `-InstallDeps` | 构建前端前执行 `npm install` |
| `--gtk3` | `-Gtk3` | Linux GUI 改用 GTK3 + webkit2gtk-4.1 |
| `--wails` | `-Wails` | 用 Wails CLI 构建 GUI（默认 `go build`） |
| `--silent` | `-Silent` | 不交互 |
| `--help` / `-h` | `-Help` | 显示帮助 |

Windows 端 PowerShell 对同一个命名参数只绑定一次，重复取值请用数组语法（`-Type cli,gui`）；`build.sh` 支持 POSIX 重复写法（`--type cli --type gui`）。未指定的维度默认取全部。

### 示例

```bash
# Linux / macOS / WSL
./build.sh --all                                              # 全部 12 个目标
./build.sh --type cli --all                                   # 全部 7 个 CLI
./build.sh --platform windows --arch arm64 --type cli --type gui
./build.sh --ci --platform linux --arch arm64 --type cli --type gui
./build.sh --dry-run --all                                    # 只看计划
```

```powershell
# Windows
.\build_windows.ps1 -All
.\build_windows.ps1 -Type cli -All
.\build_windows.ps1 -CI -Platform windows -Arch arm64 -Type cli,gui
.\build_windows.ps1 -Platform windows -Arch x64 -Type gui -InstallDeps -BuildMsix
.\build_windows.ps1 -DryRun -All
```

### CI：ARM64 必须原生构建

普通 CI（build.yml，push/PR）只做**构建 + 冒烟**，不做打包/上传。GUI 矩阵按"每作业一个平台/架构"拆分，ARM64 GUI 只在原生 ARM 运行器上编译；CLI 是纯 Go（`CGO_ENABLED=0`），7 个目标统一由 `cli-build` 单作业交叉产出（不涉及任何工具链），因此没有重复构建。打包（MSIX/7z/tar.gz）只在发布流水线（tag/手动触发）里做。

| GUI 目标 | 运行器 | 说明 |
| --- | --- | --- |
| `linux/arm64` | `ubuntu-24.04-arm` | 原生 ARM64 Linux 运行器 |
| `linux/x64` | `ubuntu-latest` | 原生 x64 Linux |
| `windows/x64/x86/arm64` | `windows-latest` | Go 以 `CGO_ENABLED=0` 直接产出三个架构；GitHub 提供原生 Windows ARM64 运行器后可切换 |

`--ci` / `-CI` 下脚本不会导出任何 `CC`/`CXX`，因此 ARM64 任务的日志中不会出现 `aarch64-linux-gnu-gcc` 或 `osxcross`；CI 会额外检索这两个关键字，命中即判定失败。Windows GUI 的版本资源在每次链接前按目标架构用 go-winres 现场生成（`--arch amd64/386/arm64`，命名 `rsrc_windows_<arch>.syso`），仓库不保留任何裸 syso——因此 Linux/Darwin 构建永远不会链接 Windows 资源对象。

### 交互式模式

不带任何选择参数（且未加 `--silent` / `--ci`）运行时进入向导：选择语言 → 是否安装依赖 → 构建类型 → 平台 → 架构 → 是否允许交叉编译 → 确认构建计划。

### 产物校验

```bash
file build/bin/cli/Linux/arm64/snishaper      # ELF aarch64
file build/bin/gui/Linux/arm64/SniShaper      # ELF aarch64
```

Windows 上可用 `dumpbin /headers build\bin\cli\Windows\arm64\snishaper.exe`，或读取 PE 头 machine 字段（`0x8664`=x64、`0xAA64`=arm64、`0x014C`=x86）。

---

## Windows 构建

```powershell
# 克隆仓库
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# 完整编译（交互模式，自动安装依赖、可选 MSIX 打包）
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# 或使用 PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1
```

### 构建脚本命令行参数

`build_windows.ps1` 支持以下参数，可跳过交互式选择：

| 参数 | 可选值 | 说明 |
| -------------- | ------------------------------ | ------------------------------------------------------------ |
| `-Build` | `<系统> <运行方式> <构建范围>` | 三段式构建参数。**系统**：`windows` / `linux` / `all`；**运行方式**：`gui` / `cli` / `all`；**构建范围**：`frontend` / `backend` / `all`。省略时进入交互式菜单；旧格式 `-Build frontend/backend/all` 仍向后兼容 |
| `-Lang` | `en` / `cn` / `ru` | 指定脚本界面提示语言，省略时默认英文 |
| `-Arch` | `x64` / `arm64` / `x86` | 目标架构（接受 amd64/386 别名），默认跟随宿主系统。`x86` 仅构建 Windows——Linux（含 CLI）与 Darwin 不构建 |
| `-InstallDeps` | 无值（开关） | 构建前端前执行 `npm install` 安装 npm 依赖 |
| `-BuildMsix` | 无值（开关） | 编译完成后生成 MSIX 安装包（需要 WinApp CLI） |
| `-SkipSign` | 无值（开关） | 跳过 MSIX 签名，生成的文件添加 `unsigned_` 前缀（需配合 `-BuildMsix`） |
| `-Cli` | 无值（开关） | 额外构建 headless CLI（`-Arch` 决定目标平台），输出到 `build/bin/cli/<Platform>/<Arch>/` |
| `-Gtk3` | 无值（开关） | Linux（WSL）构建时改用 GTK3 + webkit2gtk-4.1（等价于 `build.sh --gtk3`） |
| `-Silent` | 无值（开关） | 静默模式，跳过所有交互提示；省略 `-Build` 时默认 `windows gui all`，省略 `-Lang` 时默认 `en` |

**行为说明：**

- **自动提权**：脚本需要管理员权限。若以普通用户运行，会通过 UAC 弹窗自动提权重启自身，并以原样传递所有参数。
- **预清理**：构建开始前会强制结束正在运行的 `snishaper` 进程，避免文件占用。
- **版本同步**：后端编译前从 `Package.appxmanifest` 读取版本号与发布渠道，经 go-winres 同步到版本资源并通过 ldflags 注入；若 go-winres 失败则保留现有版本资源继续构建。后端始终执行 `go mod download`。
- **MSIX 打包**：依赖 WinApp CLI（`winget install Microsoft.WinAppCLI`）；缺少 `devcert.pfx` 证书时会自动从 manifest 生成并安装证书。已构建的 Windows GUI 架构（`-Arch x64,x86,arm64`）全部作为负载，产出单个混合架构包，输出到 `build/bin/gui/Windows/<name>_<version>_<arch...>.msixbundle`（仅单架构时退化为同名 `.msix`）。
- **Linux 构建（WSL）**：`-Build linux` 通过 WSL 调用 `build.sh`（默认 GTK4，`-Gtk3` 切换 GTK3，`-Arch` 以 `--arch` 透传）；未检测到 WSL 时输出警告并跳过 Linux 构建。

### 用法示例

```powershell
# Windows GUI，构建全部（前端 + 后端）
.\build_windows.ps1 -Build windows,gui,all

# Windows GUI，仅构建前端
.\build_windows.ps1 -Build windows,gui,frontend

# Linux GUI（通过 WSL），构建全部
.\build_windows.ps1 -Build linux,gui,all

# 仅构建 CLI（跨平台 headless）
.\build_windows.ps1 -Build windows,cli,all

# 同时构建 Windows + Linux GUI
.\build_windows.ps1 -Build all,gui,all

# 构建全部平台 GUI + CLI
.\build_windows.ps1 -Build all,all,all

# arm64 构建 + MSIX 打包
.\build_windows.ps1 -Build windows,gui,all -Arch arm64 -BuildMsix

# 静默模式（CI/CD 适用，无交互）
.\build_windows.ps1 -Silent

# 旧格式仍向后兼容
.\build_windows.ps1 -Build frontend -Lang cn
.\build_windows.ps1 -Build all -BuildMsix -SkipSign

# 无参数 = 交互模式
.\build_windows.ps1
```

---

## Linux 构建

Linux 构建使用统一的 `build.sh`（交互式菜单：GUI / CLI / 全部），在 Linux 本机（或 Windows 上的 WSL2）执行；Windows 用户只需运行 `build_windows.ps1`，无需关心 GTK 依赖。

### 依赖（Ubuntu / Debian）

```bash
# GTK4 + WebKitGTK 6.0（默认，仅 GUI 需要；CLI 构建无需 GTK）
sudo apt-get update
sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev

# 或使用 GTK3 + webkit2gtk-4.1
# sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

### 构建命令

```bash
# 克隆仓库
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# 交互式菜单（1 GUI / 2 CLI / 3 GUI+CLI + 架构选择）
./build.sh

# GUI（使用已有的 frontend/dist）
./build.sh --gui

# 先构建前端再编译 GUI
./build.sh --with-frontend

# 使用 GTK3 + webkit2gtk-4.1
./build.sh --gtk3

# 仅构建 CLI（headless，跨平台）
./build.sh --cli

# GUI + CLI 一起构建
./build.sh --all

# 指定架构（GUI/CLI 通用）
./build.sh --gui --arch arm64

# x86 仅构建 Windows CLI
./build.sh --cli --arch x86
```

构建产物：GUI 输出 `build/bin/gui/Linux/<arch>/SniShaper`（含 `rules/`、`config/` 种子文件，TUN / 系统代理需要 root，运行时 `sudo ./build/bin/gui/Linux/x64/SniShaper`）；CLI 输出 `build/bin/cli/<Platform>/<arch>/snishaper[.exe]`。

---

## 版本与发布渠道

版本号与发布渠道（`release` / `beta` / `alpha` / `rc`）由项目根目录的 `Package.appxmanifest` **统一提供**：

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

Windows 与 Linux 构建均从此文件读取版本信息，并通过 ldflags 注入（`snishaper/app.buildVersion`、`snishaper/app.buildChannel`）。仓库中不存在独立的版本 JSON 文件。

---

## 开发环境

- `Go 1.27+`
- `Node.js 24+` / `npm 11+`
- Windows：MSVC 工具链（Wails v3）、WinApp CLI（MSIX 打包）
- Linux：GTK4 / WebKitGTK 或 GTK3 开发包（见上）
- TUN 模式依赖 gvisor 网络栈（Windows 通过 `with_gvisor` 构建 tag 启用）

构建产物：

- 前端资源位于 `frontend/dist`
- Windows GUI 位于 `build/bin/gui/Windows/<arch>/snishaper.exe`（默认 x64）
- Linux GUI 位于 `build/bin/gui/Linux/<arch>/SniShaper`
- CLI 位于 `build/bin/cli/{Windows,Linux,Darwin}/<arch>/snishaper[.exe]`

---

## 持续集成

双平台 CI 流水线：

- **`build.yml`**：每次 push / PR 触发，在 `windows-2025` 上构建 Windows、在 `ubuntu-24.04` 上构建 Linux，并执行编译与二进制冒烟验证。
- **`_release_pipeline.yml`**：发布流水线。Windows runner 产出 MSIX 与 `snishaper-windows-amd64.7z` 便携包，Ubuntu runner 产出 `snishaper-linux-amd64.tar.gz`，最后在 Windows runner 合并两个平台的产物并创建 GitHub Release。Release notes 优先由 runner 本地 Ollama（默认 `qwen3.5:2b`）生成摘要；Ollama 不可用时降级为分类 commit 列表。

---

## 跨平台说明

Windows 与 Linux 由同一仓库构建，平台相关实现通过 Go build tags 隔离（如 `//go:build linux` / `windows`）。无需再访问独立的 Linux 仓库。

CLI（headless）版本作为本仓库的 `cli/` 子目录维护，与 GUI 共用同一套核心代码与版本机制（`Package.appxmanifest`），由 `build.sh --cli` / `build_windows.ps1 -Cli` 构建，CI 与发布流水线同时产出 GUI 与 CLI 产物。
