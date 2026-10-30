# Linux 打包（deb / RPM / AppImage）

除现有的压缩包分发外，SniShaper 提供 Linux 发行版软件包。打包定义在 `packaging/`，CI 工作流为 `.github/workflows/linux-packages.yml`（手动触发，不阻塞合并）。

## 格式评估结论

| 格式 | 结论 | 理由 |
| --- | --- | --- |
| `.deb` | ✅ 提供 | Debian/Ubuntu 及衍生版覆盖面最大；nfpm 单配置产出，维护成本最低 |
| `.rpm` | ✅ 提供 | Fedora/RHEL/openSUSE 生态；与 deb 共用同一份 nfpm 配置，边际成本接近零 |
| AppImage | ✅ 提供 | 免安装、单文件、适合不在包管理器覆盖内的发行版；只含 GUI（CLI 走 deb/rpm 或压缩包） |
| Flatpak / Snap | ❌ 暂不提供 | 沙箱与 TUN/系统代理能力冲突（需要 high-privilege 接口与宿主网络栈），维护成本高 |

## 支持的发行版

GUI 为 Wails v3（GTK4 + WebKitGTK 6.0），运行时依赖：

| 发行版 | 最低版本 | 运行时库 |
| --- | --- | --- |
| Ubuntu | 24.04 LTS | `libgtk-4-1`、`libwebkitgtk-6.0-4` |
| Debian | 13 (trixie) | `libgtk-4-1`、`libwebkitgtk-6.0-4` |
| Fedora | 40+ | `gtk4`、`webkitgtk6.0` |

- deb 包硬依赖 `libgtk-4-1`，WebKit 因各发行版包名不一致列为 recommends（缺失时 GUI 启动会报错并提示缺库，CLI 不受影响）。
- RPM 依赖按 Fedora 命名；openSUSE 用户请用压缩包或自行安装对应库。
- AppImage 自带依赖，不需要安装 GTK/WebKit 运行库，但 **TUN 模式仍需 root** 与 `/dev/net/tun`。
- 架构：`amd64` 与 `arm64`。
- 所有格式都需要 **管理员权限** 才能启用系统代理与 TUN；普通代理模式（浏览器手动指向 127.0.0.1:8080）无需提权。

## 包内容（deb / rpm）

| 文件 | 说明 |
| --- | --- |
| `/usr/bin/SniShaper` | GUI 主程序 |
| `/usr/bin/snishaper` | 无头 CLI |
| `/usr/share/applications/snishaper.desktop` | 桌面入口（含 zh_CN / ru 本地化注释） |
| `/usr/share/icons/hicolor/512x512/apps/snishaper.png` | 应用图标 |

## 升级与卸载行为

- **升级**：`apt install ./snishaper_<版本>_amd64.deb`（或 `dnf upgrade ./snishaper-<版本>.rpm`）直接覆盖 `/usr` 下的包属文件，无需卸载旧版。
- **卸载**：`apt remove snishaper` / `dnf remove snishaper` 只删除包属文件。
- **用户数据永不被包管理器触碰**：配置、日志、规则与证书全部位于 `~/.config/snishaper`，升级与卸载均不改动；确认不再使用时可手动删除该目录。
- AppImage 是单文件：删除即卸载，同样不触碰 `~/.config/snishaper`。

## 本地构建

```bash
# 前置：go、node、GTK4/WebKitGTK 开发包（见 docs/build.md）
./build.sh --platform linux --arch x64 --type cli --type gui
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
./packaging/fetch-linuxdeploy.sh amd64
./packaging/build-packages.sh amd64 <版本号>
# 产物：build/dist/linux/amd64/ 下的 .deb / .rpm / .AppImage
```

CI：Actions 页面选择 `linux-packages` 工作流手动运行，按架构产出制品（artifacts）并自动校验 deb 内容完整性（二进制、desktop、图标齐全）。
