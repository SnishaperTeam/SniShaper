# Implementation Plan: TUN SNI 模块演进（sni_parser / SNI 重写 / 启停稳定性）

**Input**: Feature specification from `spec/tun-sni-module/spec.md`

## Summary

演进现有 `pkg/singtun`：新增独立 SNI 解析模块（ECH 检测/分片/畸形安全）与 ClientHello SNI 重写模块；Handler 按"惰性嗅探 + 规则决策回调"接入现有规则引擎（`Rule.SniFake` 作为重写目标，正则匹配复用现有 `~` 语法）；修复启停稳定性（启动前显式残留清理阶段、关闭路径超时上限、Release 收敛上限、内存 instrumentation）；模块内引入 slog 并保留 logf 桥接。

## Technical Context

**Language/Version**: Go 1.27（go.mod 现状，满足 >=1.23 要求）
**Primary Dependencies**: `sagernet/sing-tun v0.9.6`（TUN 设备 + AutoRoute + DNSHijack + 栈装配）、`sagernet/gvisor`（网络栈，经 sing-tun 间接引入）、`sagernet/sing`、`miekg/dns`、`log/slog`（标准库）
**State Management**: N/A（Go 后端模块）
**Storage**: 无新增持久化；规则沿用 `rules/config.json` + `config/settings.json`
**Testing**: 标准 `go test`（仓库现状，不引入框架）
**Target Platform**: Windows 为主（用户复现环境，wintun）；macOS/Linux 保持编译与行为不回归
**Project Type**: 桌面应用（Wails v3）的 Go 后端模块
**Performance Goals**: 单次启停内存峰值增量数百 MB 以内；Stop/Shutdown 最坏 ≤60s 返回；TUN 路径 TCP 转发不引入可感知延迟（惰性嗅探仅在需要时读取首包）
**Constraints**: 禁止 CGO；禁止 songgao/water、xray/v2ray、sing-box 本体；禁止手写 TCP 状态机/NAT；TUN 异常不得 panic 主进程
**Scale/Scope**: `pkg/singtun`（~10 文件）+ `proxy`（规则决策接口）+ `core`（装配点）局部改动

## Project Structure

### Documentation (this feature)

```text
spec/tun-sni-module/
├── spec.md              # 需求规范
├── plan.md              # 本文件
└── tasks.md             # 任务拆解（Phase 3 产出）
```

### Source Code (repository root)

```text
pkg/singtun/
├── manager.go            # [修改] 启动前残留清理阶段；closeWithTimeout 有界等待；Release 轮次上限；slog
├── handler.go            # [修改] 惰性 SNI 嗅探接入决策回调；重写注入（prefixConn）；UDP/DNS 路径不动
├── sni_parser.go         # [新增] TLS ClientHello 解析：SNI 提取、ECH 检测、畸形安全、连接包装回放
├── sni_parser_test.go    # [新增] 标准/分片/畸形×N/ECH/非TLS 单测
├── sni_rewrite.go        # [新增] ClientHello SNI 重写：各层长度重算、记录重组
├── sni_rewrite_test.go   # [新增] 等长/变长/ECH 跳过/结构完整性单测
├── lifecycle_test.go     # [新增] Release 收敛上限、幂等 Stop、清理阶段可测逻辑
├── logger.go             # [修改] slog Logger + 现有 logf 桥接保留
├── adapter_windows.go    # [微调] cleanupStaleAdapters 汇报移除数量供启动阶段日志
└── (其余文件不动)

proxy/
├── proxy.go              # [修改] 新增面向 TUN 的 SNI 决策方法（包装 matchRule + SniFake）
└── rules 相关测试        # [新增] ~ 正则匹配、SniFake 决策的单测

core/
└── core_runtime.go       # [修改] 装配点：向 singtun 注入 SNI 决策回调
```

**Structure Decision**: 遵循现有仓库架构（Go 标准布局、`pkg/` 分包），不引入新目录层级。用户要求的"tun_manager / sni_parser / rule_engine / handler 分离"映射为：`manager.go`（tun_manager）、`sni_parser.go` + `sni_rewrite.go`（sni_parser）、`proxy` 现有规则引擎（rule_engine，复用）、`handler.go`（handler）。新增文件仅 4 个（含 2 个测试），改动面最小。

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| 新增 sni_parser 而非扩展 pkg/tlsfrag.ParseClientHello | 解析器需要 ECH 检测与结构化模型（供重写），tlsfrag 返回的是分片偏移（供分片发送）；两者消费者与演化方向不同 | 扩展 tlsfrag 需改其签名并波及 proxy/tls_fragment.go 调用方；且 singtun 依赖 proxy 已存在，再依赖 tlsfrag 会让分片逻辑与解析逻辑耦合 |
| Handler 注入 SNI 决策回调（func 类型）而非直接传 RuleManager | 避免 singtun 深度耦合 proxy 内部类型；回调签名极小（SNI → 重写目标），单测可用闭包伪造 | 直接传 `*proxy.RuleManager` 使 Handler 测试必须构造完整规则管理器；matchRule 为非导出方法还需额外导出 |

## Research & Decisions

### D1: 网络栈与设备层（库名修正）
- **Decision**: 沿用 `sagernet/sing-tun`（设备 + AutoRoute + DNSHijack + gvisor 栈装配），gvisor 栈经 sing-tun 的 `NewStack("gvisor", ...)` 使用，不触碰 gVisor 原生 fdbased
- **Rationale**: 需求中 `github.com/sagernet/netstack` 不存在；sing-tun 是该需求的实际对应物，已在 go.mod 且 manager.go 已验证可用
- **Alternatives considered**: gVisor 原生 fdbased（用户明确排除）；songgao/water + 手写栈（废弃且违反"不手写状态机"）

### D2: SNI 解析器为 singtun 内独立模块（不扩展 tlsfrag）
- **Decision**: 新增 `pkg/singtun/sni_parser.go`，自包含解析 TLS record → handshake → ClientHello → extensions，输出结构化结果（SNI、ECH 标记、失败原因）；从 `io.Reader`（bufio 包装的 conn）读取并返回包装连接保证已读字节回放
- **Rationale**: 现有 `handler.go` 内联解析无 ECH 检测、无单测；tlsfrag 面向分片偏移，接口形态不同
- **Alternatives considered**: 扩展 `tlsfrag.ParseClientHello`（见 Complexity Tracking）；引入 utls 解析（重型依赖，违反约束）

### D3: SNI 重写目标来源 = `Rule.SniFake`
- **Decision**: 规则命中且 `SniFake` 非空 → 以 SniFake 为目标重写；`SniFake` 为空 → 不重写原样透传。决策经 ProxyServer 新增的导出方法包装 `matchRule` 获得
- **Rationale**: `sni_fake` 字段已存在于规则 schema（proxy.go Rule/SiteGroup），与用户确认的"规则条目级配置"一致；现有代理链路（MITM/上游拨号）已消费同一字段，TUN 层与之语义统一
- **Alternatives considered**: 新增全局配置项作回退（当前无全局 SniFake 配置项，YAGNI，规则级为空即明确语义"不伪装"）

### D4: ECH 场景行为
- **Decision**: 检测到 ECH 扩展（encrypted SNI 存在）→ 用外层 SNI 参与规则匹配（降级），**跳过重写**（改外层 SNI 会破坏 ECH 完整性校验），日志标记
- **Rationale**: inner SNI 加密不可见是协议事实；重写 outer SNI 将导致 ECH 验证失败反而断连，"降级匹配 + 不动字节"是唯一安全路径；符合 spec FR-006/边界条款
- **Alternatives considered**: 同时重写 ECH config list（超出"不引入重型依赖"边界，需 ECH 密钥协商，明确不做）

### D5: 惰性嗅探条件
- **Decision**: 仅在以下情形嗅探 ClientHello：(a) fake-ip 反查得到的是 IP（无域名，现有行为）；(b) 按已知域名匹配规则后 `SniFake` 非空（需要改写字节）。其余流量不读首包，零额外延迟
- **Rationale**: 全量嗅探给明文/非 TLS 流量加 3s 超时读等待，收益为零；用户需求"读取 ClientHello"只对需要决策的流有实际意义
- **Alternatives considered**: 所有 TCP 流全量嗅探（延迟代价不可接受）

### D6: 启停稳定性——关闭路径全部有界
- **Decision**: ① `closeWithTimeout` 超时后不再无限期 `<-done`：设硬上限（目标 30s），到顶即记录泄漏告警并继续后续清理——残留设备交由"启动前残留清理"兜底；② `Handler.Release` 快照循环设最大轮次（目标 5 轮），到顶记录剩余连接数并返回；③ Stop/Shutdown 幂等语义维持现状（按资源残留判断）
- **Rationale**: 现状 `closeWithTimeout` 超时后无界等待且 Stop 持锁 → wintun Close 挂死即整机冻结（用户复现的"卡死"最可能根因）；"必须等 Close 返回才能删适配器"的旧约束改由下次启动前的显式清理阶段承担
- **Alternatives considered**: 超时后强杀 goroutine（Go 无安全手段）；超时后直接删适配器与 Close 竞争句柄（旧注释已论证会互相抢占）

### D7: 启动前残留清理为显式阶段
- **Decision**: `Manager.Start` 最前面增加 `cleanupResiduals` 阶段，顺序为：等待上一次释放完成（现有 `waitReleasingLocked`，维持 12s 上限）→ 若仍有残留资源执行一次释放 → Windows 下调用 `cleanupStaleAdapters`（从 `newTunWithRetry` 内部提升为显式阶段并输出移除数量）→ 阶段耗时日志。整个阶段与后续创建流程同样受 defer 回滚保护
- **Rationale**: 用户明确要求"启动时先清理残留再启动"；现状清理藏在重试函数内部且仅在 Windows 首次尝试前执行，崩溃遗留适配器的清理时机不可见、不可验证
- **Alternatives considered**: 仅保留 newTunWithRetry 内部调用（不满足"显式步骤 + 可验证"需求）

### D8: 内存飙升——instrumentation 先行，修复随测量落地
- **Decision**: 实现阶段先加轻量 instrumentation：启停各阶段记录进程 RSS（`runtime.MemStats` 或系统 API）与 Handler 存活连接数、Release 各轮剩余数；随后在本地复现启停循环定位无上限增长点（候选：D6 已覆盖的冻结路径连带、gvisor 栈未释放累积、live map 泄漏）。修复必须以测量证据为准，不做无依据的猜测性重构
- **Rationale**: 数十 GB 尖峰在静态审查中无法唯一归因（现有代码无显式无上限分配）；盲改风险大于收益
- **Alternatives considered**: 直接猜测性修复（无法验证是否命中根因）

### D9: slog 范围与桥接
- **Decision**: `pkg/singtun` 内部改用 `*slog.Logger`（分级 debug/info/warn/error，启停阶段带耗时属性），`NewManager` 保留 `logf` 参数并内部桥接为 slog（App 层零改动）；`singTunLogger`（sing-tun 适配）改为输出到 slog
- **Rationale**: spec FR-010 要求分级日志；保留 logf 桥接使 `core_runtime.go` 装配点与 App 日志管道无需迁移
- **Alternatives considered**: 全仓库迁移 slog（范围爆炸，明确不做）

### D10: 正则规则——复用现有 `~` 语法
- **Decision**: 不新增正则实现。现有 `domainMatchScore` 已支持 `~pattern` 正则（编译缓存 + 非法模式返回不匹配），TUN 决策方法经 `matchRule` 自动获得该能力；补充正则匹配单测（含非法正则跳过用例）锁定行为
- **Rationale**: 功能已存在且行为符合 FR-008（非法正则不影响其他规则）；重复实现违反复用原则
- **Alternatives considered**: 新增独立正则匹配层（纯重复）

### D11: 决策回调装配
- **Decision**: `Handler` 增加 `SetSNIDecider(func(sni string) (rewriteTo string, matched bool))`；`core_runtime.go` 在创建 Manager 后注入由 `ProxyServer` 新增导出方法实现的回调。回调为 nil 时行为等同现状（永不重写）
- **Rationale**: 最小耦合（见 Complexity Tracking）；nil 回调保证不装配规则引擎时（测试/独立使用）Handler 行为完全兼容
- **Alternatives considered**: Handler 直连 RuleManager（耦合过大）

## Data Model

### SNIInfo（sni_parser 产物）
- `IsTLS bool` — 首字节是否 TLS handshake record
- `SNI string` — 外层 server_name（可空）
- `HasECH bool` — 是否存在 ECH 扩展（encrypted SNI）
- `Err error` — 畸形/读取失败原因（非 panic）

### SNIDecision（决策回调产物，proxy 侧实现）
- `RewriteTo string` — 重写目标（= 命中规则的 SniFake；空 = 不重写）
- `Matched bool` — 是否命中启用中的规则（用于日志）

### prefixConn（handler 内部）
- 语义：`Read` 先消费注入的前缀字节（重写后的 ClientHello），耗尽后透传底层连接；`Write` 直通。保证重写字节先于客户端后续数据到达上游

### RewriteResult（sni_rewrite 产物）
- `Record []byte` — 重写后的完整 TLS record（各层长度已重算）
- `Err error` — 长度越界/结构损坏等原因

### 生命周期度量（manager 内部，仅日志）
- 各阶段耗时（现有模式延续）、清理移除的适配器数、Release 各轮剩余连接数、阶段边界 RSS

## Contracts & Interfaces

### 对外（App 层，签名不变）
- `singtun.NewManager(resolver *dohresolver.FailoverResolver, logf func(string)) *Manager` — 不变
- `(*Manager).Start(cfg proxy.TUNConfig, proxyAddr string) error` — 不变；内部新增前置清理阶段
- `(*Manager).Stop() error` / `(*Manager).Shutdown() error` / `(*Manager).Status() proxy.TUNStatus` — 不变；Stop/Shutdown 保证有界返回

### 新增（模块间）
- `sni_parser`: `SniffClientHello(conn net.Conn, timeout time.Duration) (SNIInfo, net.Conn)` — 返回的 conn 总是包装后的（已读字节回放）；超时/非 TLS 返回 IsTLS=false 且 conn 仍可用
- `sni_parser`: `ParseClientHelloRecord(record []byte) (SNIInfo, error)` — 纯函数，供单测
- `sni_rewrite`: `RewriteClientHelloSNI(record []byte, newSNI string) ([]byte, error)` — 纯函数；ECH record 返回明确错误（调用方跳过重写）
- `Handler.SetSNIDecider(fn func(sni string) (rewriteTo string, matched bool))` — 装配注入；nil 合法
- `ProxyServer`（proxy 包）: `SNIRewriteDecisionForTUN(host string) (rewriteTo string, matched bool)` — 包装 matchRule（mode=tun 语境）与 SniFake 提取

### 约束
- 解析器对任何输入不得 panic（畸形输入返回 Err）；所有 `Read` 均带超时并由调用方恢复 deadline
- 重写后的 record 必须通过"再次解析 SNI == 新值"的自洽校验（单测断言）
- 决策回调在数据面热路径调用，实现必须无锁竞争热点（复用 matchRule 的 RLock 现状即可）
