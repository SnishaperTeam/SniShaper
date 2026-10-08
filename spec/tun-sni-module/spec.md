# Feature Specification: TUN SNI 模块演进（sni_parser / SNI 重写 / 启停稳定性）

**Created**: 2026-10-07  
**Status**: Draft  
**Input**: 用户描述："从零实现核心 TUN 模块" + 五项澄清选择 + 修正："启动和关闭 TUN 时内存会飙升至数十 GB 并卡死冻结，启动 TUN 时需先清理残留再启动"

## Overview

演进现有 `pkg/singtun` TUN 模块（而非重写）：将 SNI 解析抽为独立可测模块并补齐 ECH/分片/畸形包边界处理；新增 TUN 层 SNI 重写透传能力并接入现有规则引擎（补正则支持）；修复 TUN 启停过程中的内存飙升与冻结问题，并在启动前增加残留清理步骤。上游沿用本地 HTTP 代理 CONNECT 架构不变。

## User Scenarios & Testing *(mandatory)*

### User Story 1 - 启动前残留清理 (Priority: P0)

用户启动 TUN 时，系统必须先清理上次运行残留的资源（上次异常退出遗留的虚拟网卡、未完全释放的旧 TUN 实例/网络栈/连接），清理完成后才开始创建新 TUN 设备。反复启停或崩溃后重启，不会因残留资源累积而导致启动失败、内存异常或路由错乱。

**Why this priority**: 用户实测报告启停时内存飙升至数十 GB 且系统冻结；残留累积是启动路径上的直接诱因，必须最先解决，否则其余功能无法可靠验证。

**Independent Test**: 崩溃/强杀进程后重新启动 TUN，观察启动日志出现残留清理步骤、启动成功、旧虚拟网卡被移除；可通过"启动→强杀→再启动"循环独立验证。

**Acceptance Scenarios**:

1. **Given** 上次进程异常退出遗留 SniShaper 虚拟网卡，**When** 用户启动 TUN，**Then** 启动流程先执行残留清理（日志可见），移除遗留网卡后才创建新设备，启动成功
2. **Given** 上一次 Stop 尚未完全释放（仍在释放窗口内），**When** 用户再次启动 TUN，**Then** 系统等待释放完成或明确报错，不会在旧资源未清空时叠加创建新实例
3. **Given** 正常运行中，**When** 用户启动 TUN（重复启动），**Then** 系统幂等处理，不创建第二个设备实例

---

### User Story 2 - 启停稳定（内存有界、不冻结） (Priority: P0)

用户启动或关闭 TUN 时，应用内存占用保持在正常范围（不出现 GB 级尖峰），且整个启停流程在有限时间内完成，界面与主流程不冻结卡死。任何清理/关闭路径都必须有超时上限，不能无限期持锁等待。

**Why this priority**: 与 Story 1 同源的用户报告核心痛点；内存飙升+冻结直接导致整机不可用，是回归性最强的质量需求。

**Independent Test**: 连续执行启停循环（≥20 次），监控进程 RSS 与单次启停耗时；期间 UI 可响应。可独立于 SNI 功能验证。

**Acceptance Scenarios**:

1. **Given** TUN 正在运行且有活跃流量，**When** 用户关闭 TUN，**Then** 关闭流程在有限时间内返回（即使底层设备关闭挂死也不无限阻塞主流程），内存回落到 TUN 运行前水平附近
2. **Given** 连续启停 20 次，**When** 观察进程内存，**Then** RSS 无单调增长趋势，单次启停内存峰值增量保持在数百 MB 以内（不再出现数十 GB 尖峰）
3. **Given** 底层 TUN 设备关闭操作挂死，**When** 执行 Stop/Shutdown，**Then** 主流程在超时上限后继续完成其余清理并返回，应用不冻结
4. **Given** 启停过程中任一环节出错，**When** 流程结束，**Then** 已分配的资源被回滚释放，无 goroutine/连接/网卡泄漏

---

### User Story 3 - SNI 解析模块化（含 ECH/分片/畸形边界） (Priority: P1)

TLS ClientHello 的 SNI 解析从现有 Handler 中抽出为独立模块：能正确解析标准 ClientHello；能容忍跨 TCP 段分片到达的 ClientHello；对畸形包（截断、类型错误、长度越界）安全返回失败而非崩溃；识别 ECH 场景并降级使用外层（outer）SNI 参与规则匹配。已读取的字节通过包装连接完整回放，不丢失客户端数据。

**Why this priority**: SNI 是全部规则匹配与伪装动作的数据基础；独立模块 + 单测是后续重写能力的地基。

**Independent Test**: 用标准 `go test` 对解析模块注入构造的 ClientHello 字节流（正常/分片/畸形/ECH）独立验证，不需要真实 TUN 设备。

**Acceptance Scenarios**:

1. **Given** 一条完整的标准 TLS ClientHello 字节流，**When** 解析，**Then** 返回正确 SNI 域名
2. **Given** ClientHello 的 TLS record 头与 body 分多次到达（模拟 TCP 分片），**When** 解析，**Then** 仍返回正确 SNI
3. **Given** 畸形输入（非握手记录/截断/长度字段越界/空扩展），**When** 解析，**Then** 返回"无 SNI"错误而非 panic，连接不中断
4. **Given** 带 ECH 扩展的 ClientHello（inner SNI 加密不可见），**When** 解析，**Then** 返回外层 SNI 并标记 ECH 存在，规则匹配使用外层 SNI
5. **Given** 非 TLS 流量（如明文 HTTP），**When** 嗅探，**Then** 安全返回"非 TLS"，连接原样透传且已读字节不丢失

---

### User Story 4 - 规则匹配 + SNI 重写透传 (Priority: P1)

对每条进入 TUN 的 TCP 流：提取 SNI 后经现有规则引擎匹配（域名、后缀、正则）。命中伪装规则的流，在 TUN 层重写 ClientHello 中的 SNI 字段后透传（不终结 TLS）；未命中的流字节级原样透传。UDP 流量维持现状（透传 + 连接跟踪，不做 SNI 解析）。

**Why this priority**: 这是"伪装"能力在 TUN 层的落地；依赖 Story 3 的解析模块与现有规则引擎。

**Independent Test**: 单测中对重写函数注入原始 ClientHello 与目标 SNI，断言输出流的 SNI 字段为新值且 TLS 记录结构完整；集成验证可在本地代理端观察收到的 SNI。

**Acceptance Scenarios**:

1. **Given** SNI 命中伪装规则，**When** 流量经过 TUN，**Then** 转发到上游的 ClientHello SNI 为重写后的值，TLS 握手可正常完成
2. **Given** SNI 未命中任何规则，**When** 流量经过 TUN，**Then** 转发内容与原始流量字节级一致
3. **Given** 目标重写 SNI 长度与原 SNI 不同，**When** 重写，**Then** 输出的 ClientHello 各层长度字段（扩展长度/记录长度等）正确重算，对端可正常解析
4. **Given** 规则表中存在正则规则，**When** SNI 匹配，**Then** 正则命中行为与域名/后缀规则一致参与决策
5. **Given** UDP 流（非 DNS），**When** 经过 TUN，**Then** 维持现有透传与连接跟踪行为（回归，不劣化）

---

### User Story 5 - 日志分级（slog）与可观测性 (Priority: P2)

TUN 模块关键路径（启停阶段、残留清理、SNI 嗅探/重写决策、异常）输出分级日志，便于定位启停问题；日志不泄露用户访问的完整域名以外的敏感内容，异常路径有明确错误日志而非静默失败。

**Why this priority**: 启停问题排查依赖日志；优先级低于稳定性与核心功能。

**Independent Test**: 启停 TUN 并触发一次规则命中，检查日志输出包含各级别条目；独立于功能正确性验证。

**Acceptance Scenarios**:

1. **Given** TUN 启动，**When** 各阶段完成（清理/设备创建/路由/栈启动），**Then** 日志按阶段输出带耗时信息
2. **Given** 残留清理移除了遗留网卡，**When** 查看日志，**Then** 有明确条目记录被移除的设备
3. **Given** SNI 重写发生，**When** 查看日志，**Then** 记录原 SNI 与重写后 SNI 的决策日志

### Edge Cases

- ClientHello 横跨多个 TLS record（罕见但合法）→ 解析不崩溃，尽力解析或安全放弃
- fake-ip 反查失败（映射丢失）→ 现有 WARNING 路径保持，回退 SNI 嗅探
- 规则引擎热更新（rules/config.json 变更）时正则表达式非法 → 该条规则跳过并记日志，不影响其他规则
- Stop 与 Start 并发调用 → 串行化且幂等，无双实例
- ECH 且外层 SNI 为空/通配 → 视为未命中规则，透传
- 残留清理时其他 sing-tun 系工具（sing-box/mihomo）的网卡在运行 → 绝不误删（沿用现有严格匹配）

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: TUN 启动流程 MUST 在创建新设备前执行残留清理步骤：移除本项目遗留的虚拟网卡，并确保上一次实例的栈/Handler/监控器已完全释放后再进入创建流程
- **FR-002**: 所有关闭/清理路径（设备关闭、栈关闭、Handler 释放）MUST 设置超时上限；超时后 MUST 继续执行后续清理步骤并返回，不得无限期阻塞调用方或持锁
- **FR-003**: 启停流程 MUST 保证内存占用有界：不得因连接跟踪表、快照循环或缓冲累积出现无上限增长；连续启停不得产生资源（goroutine/连接/网卡）泄漏
- **FR-004**: Stop/Shutdown MUST 幂等且可重复调用；Start 对"已在运行"与"正在释放"两种状态 MUST 分别正确处理（前者直接成功，后者等待或明确报错）
- **FR-005**: 系统 MUST 提供独立的 SNI 解析模块：输入 TLS 字节流，输出 SNI 值、ECH 存在标记与解析失败原因；已消费字节 MUST 通过连接包装完整保留
- **FR-006**: SNI 解析 MUST 覆盖：标准 ClientHello、跨段分片到达、畸形包安全失败、ECH 外层 SNI 降级、非 TLS 流量识别；全部 MUST 有单元测试
- **FR-007**: 系统 MUST 在 TUN 层提供 SNI 重写能力：命中规则的连接重写 ClientHello SNI（含长度重算）后透传，不终结 TLS；重写函数 MUST 有单元测试
- **FR-008**: 规则引擎 MUST 支持域名、后缀、正则三类匹配，且被 TUN 路径复用；非法正则 MUST 被跳过并记录日志，不影响整体规则加载
- **FR-009**: 未命中规则的 TCP 流 MUST 字节级原样透传；UDP 流（非 DNS）MUST 维持现有透传与连接跟踪行为
- **FR-010**: TUN 模块关键路径 MUST 输出分级日志（debug/info/warn/error），启停各阶段与残留清理 MUST 有带耗时的日志条目
- **FR-011**: TUN 设备异常（创建失败/读写错误/关闭挂死）MUST 被捕获处理，不得导致进程 panic 或主流程冻结
- **FR-012**: 残留清理 MUST 严格限定仅清理本项目创建的资源，绝不影响其他 sing-tun 系工具的设备

### Key Entities

- **SNIInfo**: 一次 ClientHello 嗅探的结果——SNI 域名（可能为空）、是否 ECH、是否 TLS、失败原因
- **RewriteResult**: 一次 SNI 重写的产物——重写后的 ClientHello 字节流、是否成功、失败原因
- **Rule**: 规则表条目——匹配类型（domain/suffix/regex）、匹配值、动作（含重写目标 SNI）
- **TUNResiduals**: 启动前清理的对象集合——遗留虚拟网卡、未释放的旧实例资源（隐含实体，不直接暴露）
- **TUNStatus**: 现有运行状态快照——运行中/已启用/驱动/消息（沿用现有实体）

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 连续启停 TUN ≥20 次循环后，进程 RSS 无单调增长；单次启停内存峰值增量保持在数百 MB 量级以内（消除数十 GB 尖峰）
- **SC-002**: Stop/Shutdown 在最坏情况下于超时上限内返回（目标 ≤60 秒），期间应用界面保持可响应
- **SC-003**: SNI 解析单元测试覆盖 ≥6 类输入（标准/分片/畸形×多种/ECH/非 TLS）全部通过；畸形输入不产生 panic
- **SC-004**: 规则命中后上游收到的 ClientHello SNI 与规则配置一致；未命中流量与原始流量逐字节一致（测试断言）
- **SC-005**: `go build ./...` 与 `go test ./pkg/... ./proxy/...` 全部通过
- **SC-006**: 强杀进程后重启，残留清理步骤在启动日志中可见，遗留网卡被移除，启动成功率 100%（≥5 次试验）

## Assumptions

- 演进现有 `pkg/singtun`（用户已确认），不重写已验证的 manager/UDP/DNS 代码；原始需求中的"go.mod 完整内容/独立 main.go 演示/目录树交付"不再适用（模块在主程序内组装，`main.go` 已有装配路径）
- 库名修正：需求中的 `github.com/sagernet/netstack` 实际为已引入的 `sagernet/sing-tun`（设备+AutoRoute+DNSHijack）与 `sagernet/gvisor`（网络栈），技术栈不变
- 上游沿用本地 HTTP 代理 CONNECT（用户已确认）；规则引擎复用现有 `proxy` 规则体系并补正则（用户已确认）
- "ECH/ESNI 降级"界定为：inner SNI 加密不可见时，使用 outer SNI 参与匹配并在日志/结果中标记；不做 ECH 解密
- slog 引入范围限定在 TUN 模块（`pkg/singtun` 及新增解析/重写子模块），不做全仓库日志迁移；模块对外保留现有 `logf` 适配以兼容 App 层
- Windows 为主要目标平台（用户复现环境），macOS/Linux 保持编译与行为不回归
- 单元测试使用标准 `go test`（仓库现状），不引入测试框架
- 冻结根因（`closeWithTimeout` 超时后无界等待、`Release` 收敛循环、启停持锁范围）在架构阶段定案修复方案；内存飙升的精确根因允许在实现阶段用实测定位

## Open Questions

- SNI 重写的目标值来源：规则条目中直接配置目标 SNI，还是全局统一配置一个伪装域名？（当前假设：规则条目级配置，缺省回退全局值）
- 重写后是否需要配合现有 TLS 分片（tlsfrag）能力对重写后的 ClientHello 分片发送，还是保持整包发送？（当前假设：本期整包发送，分片由本地代理链路按规则另行处理）
- 内存飙升的实测基线数据（复现环境的具体数值/阶段）如用户提供，可直接用于验收比对
