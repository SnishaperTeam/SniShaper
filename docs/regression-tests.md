# 网络回归测试（Network Regression Test Suite）

本仓库网络回归测试的范围、运行方式与限制。目标：在不提权、不依赖真实 TUN 硬件的前提下，把可在 CI 稳定运行的网络状态回归用例自动化，其余用例文档化。

## CI 自动运行的用例（无提权要求）

CI（`.github/workflows/build.yml` 的 `tests` job）在 Windows 与 Ubuntu 两个平台运行：

```pwsh
go test -race -tags with_gvisor ./...
cd frontend && npx tsc --noEmit
```

覆盖范围：

| 用例 | 位置 | 说明 |
| --- | --- | --- |
| 系统代理恢复状态机（重复启停循环） | `app/app_regression_test.go` | 20 轮「启用→崩溃→恢复→干净关停」循环，验证 marker 记录/清理在端口变化下不回归 |
| 部分安装的恢复 | 同上 | 遗留 marker + 系统代理已关（系统重启场景）→ 仅清理 marker，绝不调用禁用 |
| 他人代理不被覆盖 | 同上 | marker 存在但系统代理已被其他程序改走 → 只报告，不修改 |
| 权限失败重试 | 同上 | 禁用失败（模拟无管理员权限）→ 报错并保留 marker，下次启动重试 |
| 并发决策（-race） | 同上 | 64 goroutine 并发调用纯决策函数 |
| 崩溃/中断关停/损坏 marker 决策路径 | `app/app_recovery_test.go` | 表驱动覆盖强制终止、失败启动、中断关停、系统重启四种场景 |
| 诊断导出与脱敏 | `app/app_diag_export_test.go` | 敏感字段脱敏、失败不影响运行中的代理 |
| 自更新安装、自启动、规则更新 | `app/*_test.go`、`internal/rulesguard`、`common`、`core` | 各自模块的回归 |

平台效果通过 `sysproxyStatusFn` / `sysproxyDisableFn` 接缝模拟（`app/app_system.go`）：测试永远不会改动运行机器的真实系统代理设置，只在 `t.TempDir()` 内操作 marker 文件。

## 需要提权或真实硬件的用例（手动执行）

以下用例在 CI 中**不**运行，原因与手动方法如下：

| 用例 | 原因 | 手动方法 |
| --- | --- | --- |
| TUN 启停/重启/反复循环、路由安装与清理 | 需要管理员权限 + wintun 驱动（Windows）或 `/dev/net/tun`（Linux） | 以管理员运行 GUI/CLI，反复执行 `tun on` / `tun off` 各 10+ 次观察 `tun status` 与路由表 |
| 真实 DNS 解析、IPv4/IPv6 行为 | 依赖外网连通性与运营商环境 | 开启代理后 `curl -k -m 15` 分别测试 IPv4/IPv6 站点；Linux/WSL 无头版见 `AGENTS.md` 的 TUN 测试流程 |
| 系统代理真实注册表写入/还原 | 需要管理员权限，且会改动测试机网络 | 管理员运行 GUI，反复切换「系统代理」开关，观察退出后注册表还原 |
| 长时间运行与资源占用 | CI 时长不可行 | 本机运行 24h+，观察内存（连接注册表有界，上限 4096 条）与 CPU |

## 核心库（snishaper-core）中的网络测试

引擎层测试在 `github.com/SnishaperTeam/snishaper-core` 仓库，`go test ./...` 即可运行（无提权要求）：

- `proxy/proxy_integration_test.go` — HTTP 代理端到端、Start/Stop 幂等
- `proxy/connections_test.go` — 连接注册表：注册/快照/移除、有界上限、字节计数器
- `proxy/dnsmode_test.go` — DNS 模式决策
- `proxy/tun_config_test.go` — TUN 配置校验
- `proxy/config_migration_test.go` — 配置迁移兼容
- `proxy/bench_test.go` — 性能基准（见 `docs/benchmarks.md`）
- `netiface`、`singtun`、`sysproxy` — 网卡枚举、TUN、各平台系统代理实现的单元测试
