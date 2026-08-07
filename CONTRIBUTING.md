# 贡献指南

感谢你对 **SniShaper** 的关注与支持！我们欢迎各种形式的贡献，包括但不限于代码、文档、规则、测试与反馈。

本指南将帮助你快速了解如何参与项目。

## 行为准则

请先阅读并遵守 [Code of Conduct](CODE_OF_CONDUCT.md)。我们致力于维护一个友好、包容、尊重的社区环境。

## 如何贡献

你可以通过以下方式参与：

- 提交 Bug 报告
- 提出功能建议
- 改进文档或 Wiki
- 提交代码（修复、优化、新功能）
- 完善规则配置
- 帮助测试与反馈

### 提交 Issue

请优先使用仓库已有的 Issue 模板：

- **Bug 报告**：描述问题、环境信息、复现步骤与附件
- **功能请求**：说明需求、使用场景与期望效果

提交前请先搜索是否已有相同或类似的 Issue，避免重复。

### 提交 Pull Request

1. Fork 本仓库
2. 创建新分支（建议命名清晰，例如 `fix/xxx` 或 `feat/xxx`）
3. 进行修改并确保能正常构建
4. 提交清晰的 commit 信息
5. 推送到你的 Fork 仓库
6. 向主仓库的 `main` 分支发起 Pull Request

PR 描述中请说明：

- 修改了什么
- 为什么需要这个修改
- 是否关联某个 Issue（使用 `Fixes #编号` 或 `Closes #编号`）
- 测试情况

## 开发环境

本项目基于 **Wails v3** 构建。

### 推荐环境

- Go 1.25+
- Node.js 24+
- npm 11+
- gVisor（TUN 模式需要）

### 快速构建

```powershell
# 克隆仓库
git clone https://github.com/SniShaper/SniShaper.git
cd SniShaper

# 安装前端依赖
cd frontend
npm install

# 构建前端
npm run build
cd ..

# 完整编译（推荐使用构建脚本）
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1
```

也可直接使用命令行参数控制构建行为，例如：

```powershell
# 构建前后端并安装依赖
.\build_windows.ps1 -Build all -Lang cn -InstallDeps

# 静默模式（适合 CI）
.\build_windows.ps1 -Silent
```

更多参数说明请参考 [README.md](README.md) 中的「构建与开发」部分。

构建产物：

- 前端资源：`frontend/dist`
- 可执行文件：`build/bin/snishaper.exe`

## 代码规范建议

- 保持代码风格与现有项目一致
- 提交前尽量完成基本测试（能正常启动、核心功能可用）
- 避免无关的格式化或大规模重构混入功能 PR
- 如有较大改动，建议先开 Issue 讨论

## 文档与规则

- 技术原理与使用说明主要维护在 [GitHub Wiki](https://github.com/SniShaper/SniShaper/wiki)
- 规则相关改进可直接在 `rules` 目录或通过 Issue/PR 提出
- 文档改进同样欢迎提交

## 许可

本项目采用 [MIT License](LICENSE)。你贡献的代码将默认以相同许可证授权。

## 联系与反馈

如有疑问，可通过以下方式交流：

- 直接在对应 Issue 或 PR 中留言
- 查看现有 [Issues](https://github.com/SniShaper/SniShaper/issues) 与 [Discussions](https://github.com/SniShaper/SniShaper/discussions)（如有开启）

再次感谢你的贡献！
