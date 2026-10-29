# macOS 菜单栏应用（SnishaperBar）

SniShaper 在 macOS 上以**菜单栏应用**形式交付：`macos/` 下的 Swift 包（SnishaperBar）负责状态展示与控制，实际的代理引擎由打包在应用内的 headless `snishaper` CLI 二进制承载。

## 架构

```
SniShaper.app
├── Contents/MacOS/SnishaperBar            # 菜单栏应用（Swift，SPM 构建）
├── Contents/Resources/bin/snishaper       # headless CLI（Go，代理引擎）
└── Contents/Info.plist                    # LSUIElement=true：纯菜单栏，无 Dock 图标
```

- 菜单栏应用**不实现任何代理逻辑**：启动/停止服务、代理与系统代理开关都通过执行 CLI 子命令完成（`snishaper start/stop/proxy on/off/sysproxy on/off/diag`）。
- 状态轮询使用 `snishaper status --json` 的稳定 JSON 契约（每 3 秒），不解析人类可读文本。
- TUN、规则、订阅、ECH 等全部功能由 CLI 进程提供；命令行用法见 CLI 帮助（`snishaper help`）。

## 构建

要求：macOS 13+、Xcode Command Line Tools（swift 5.9+）、Go 1.27+。

```bash
# 一键打包（编译菜单栏应用 + headless CLI 并组装 .app + zip）
macos/scripts/bundle-app.sh

# 仅编译菜单栏应用
cd macos && swift build -c release
```

产物：`build/bin/macos/SniShaper.app` 与 `SniShaper-macos-<arch>.zip`。

CI 在 Apple Silicon 与 Intel 的原生 macOS runner 上构建并打包验证（`.github/workflows/macos.yml`）。

## 安装

1. 下载 `SniShaper-macos-<arch>.zip` 并解压。
2. 将 `SniShaper.app` 拖入 `/Applications`（或 `~/Applications`）。
3. 首次运行：**构建产物未签名/未公证**，Gatekeeper 会拦截。放行方式任选其一：
   - 在 `SniShaper.app` 上右键 →「打开」→ 确认（推荐，只放行这一个应用）；
   - 或系统设置 → 隐私与安全性 → 底部出现「已阻止…」→「仍要打开」。
4. 菜单栏出现 ⬢ 图标即运行中。

## 升级

1. 退出菜单栏应用（菜单 → 退出 SniShaper）。
2. 若服务在运行，先执行 `stop`（菜单栏退出不自动停止独立的服务进程；`Contents/Resources/bin/snishaper stop`）。
3. 用新版本 `SniShaper.app` 覆盖旧版本。
4. 重新打开应用。

配置与日志按可执行文件同级目录约定存放（与 CLI 单独使用时一致），升级**不删除**任何用户数据。

## 卸载

1. 退出菜单栏应用，停止服务（见上）。
2. 删除 `SniShaper.app`。
3. 手动关闭系统代理（若启用）：系统设置 → 网络 → 代理。
4. 可选：删除配置与日志目录（应用包同级目录下 CLI 生成的 `config/`、`rules/`、`log/` 等）。

## 限制

- **未签名/未公证**：每次升级后需重新放行 Gatekeeper。
- **首次打开可能不显示授权提示**：服务进程（CLI）需要管理员权限才能开启 TUN / 修改系统代理设置；macOS 上系统代理通过 `networksetup` 生效，TUN 依赖 utun，均可能触发授权弹窗。
- 菜单栏应用当前提供核心控制（服务、代理、系统代理、诊断、日志），完整功能请配合 CLI 使用。
- 平台支持细节见 [Platform.md](Platform.md)。
