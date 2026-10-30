# 代理性能基准（Proxy Performance Benchmarks）

可重复的性能基准位于核心库 `github.com/SnishaperTeam/snishaper-core` 的 `proxy/bench_test.go`（本仓库经 go.mod 固定版本直接引用，无需单独克隆）。本文说明运行方式、覆盖的工作负载与统计方法；目标是用基准对比验证性能改动，**单次运行结果不作结论**。

## 运行方法

```pwsh
# 全部基准（-count 10 生成 benchstat 对比样本；-benchmem 必开：内存分配是数据面回归的主要信号）
go test -run '^$' -bench 'Benchmark' -benchmem -count 10 github.com/SnishaperTeam/snishaper-core/proxy/ > new.txt

# 快速冒烟（开发迭代用，结果不作对比依据）
go test -run '^$' -bench 'Benchmark' -benchmem -benchtime 100x github.com/SnishaperTeam/snishaper-core/proxy/
```

## 覆盖的代理模式与网络路径

HTTP CONNECT 与 SOCKS5 两个入口、direct 模式全链路（路由 → 拨号 → 隧道转发）。MITM/TLS 模式依赖证书与 TLS 栈，改动这些路径时以 direct 结果作对照，并在真实环境手动抽查。

## 代表性工作负载

| 基准 | 工作负载 |
|------|----------|
| `BenchmarkTunnelThroughput` | 持续传输（大文件下载/上传）：64 KiB 往返吞吐 |
| `BenchmarkTunnelShortConnections` | 短连接高频请求（API 密集型）：隧道建立 + 小载荷 + 拆除 |
| `BenchmarkTunnelConcurrent` | 大量并发连接：`RunParallel` 多并发隧道小往返 |
| `BenchmarkSOCKS5ShortConnections` | SOCKS5 客户端的短连接 churn |
| `BenchmarkRuleMatch/rules=100\|1000` | 每请求规则决策耗时（轻/重规则集热路径） |
| `BenchmarkRuleMatchProcess` / `BenchmarkFindProcessName` / `BenchmarkProcessNameInList` | 按应用分流的决策与 OS 级进程识别 |
| `BenchmarkNormalizeHost` | 每请求一次的 host 归一化 |
| `BenchmarkConnRegistryRegisterRemove` / `BenchmarkConnRegistrySnapshot` | 连接跟踪登记/注销与连接查看器快照（满载 4096 条） |

吞吐与延迟指标来自基准耗时（ns/op、B/op、allocs/op 即内存分配；CPU 时间与墙钟时间在 `-cpuprofile` 下进一步剖析）。

## 统计方法（跨版本对比）

单次基准受机器负载、温度、电源策略影响很大。对比流程：

```pwsh
go install golang.org/x/perf/cmd/benchstat@latest

# 基线（改动前）→ old.txt；改动后 → new.txt（命令同上，-count 10）
benchstat old.txt new.txt
```

判定标准：

- p 值 ≥ 0.05 视为**无显著差异**，不当作退化或优化。
- 显著变化（p < 0.05）且幅度 > 5% 时，检查分配数（allocs/op）是否同向变化再下结论。
- 性能相关 PR 附 benchstat 输出；无法复现的结果不作为合并依据。

## 环境记录

基准输出自带 `goos / goarch / cpu` 字段，随结果一并保存。对比双方必须满足：

- 同一物理机（或同型号 CI runner）、同一电源策略。
- 同一 Go 版本（`go version`）。
- 后台无其它高负载任务。

CI 不强制跑基准（共享 runner 噪声大，结果不可比）；基准对比由开发者本机或专用机器完成。

## 已知边界

- 吞吐/延迟基准走 loopback，反映的是**数据面相对开销**（缓冲、跟踪、锁），不代表公网绝对带宽。
- `BenchmarkFindProcessName` 在 macOS 上首次迭代含 lsof 冷启动，benchstat 对比时忽略首样本或用 `-benchtime` 固定次数。
- 基准针对 go.mod 固定的 core 版本运行；升级 core 后重新采集基线再对比。
