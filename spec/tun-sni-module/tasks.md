---
description: "Task list for TUN SNI module evolution"
---

# Tasks: TUN SNI 模块演进（sni_parser / SNI 重写 / 启停稳定性）

**Input**: Design documents from `spec/tun-sni-module/`
**Prerequisites**: plan.md (required), spec.md (required for user stories)

**Tests**: 用户明确要求各模块单元测试（SNI 解析覆盖分片/畸形/ECH），故包含测试任务；使用标准 `go test`。

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g. US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- 现有 Go 仓库（非单项目 src/ 布局）：改动集中在 `pkg/singtun/`、`proxy/`、`core/`
- 路径均相对仓库根 `D:\c++\Contributions\SniShaper-main`
- 构建命令在仓库根执行：`go build ./...`、`go test ./pkg/... ./proxy/...`、`.\build_windows.ps1 -Build backend -Silent`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 确认基线干净，避免把既有编译/测试问题带入本次改动

- [X] T001 基线验证：运行 `go build ./...` 与 `go test ./pkg/... ./proxy/...`，记录既有失败项（如有），确认起点状态（仓库根目录）

**Checkpoint**: 基线状态已知，后续失败可归因于本次改动

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 日志基础设施先行——后续所有任务的日志条目都经 slog 桥接输出

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T002 slog 桥接：`pkg/singtun/logger.go` 增加 `*slog.Logger` 与现有 `logf func(string)` 的双向桥接（singTunLogger 输出改走 slog 分级），Manager/Handler 内部日志调用点切换到 slog，对外 `NewManager(resolver, logf)` 签名不变（`pkg/singtun/logger.go`、`pkg/singtun/manager.go`、`pkg/singtun/handler.go`）

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - 启动前残留清理 (Priority: P0) 🎯 MVP

**Goal**: TUN 启动流程先清理残留（上次释放未完成资源、遗留虚拟网卡）再创建设备
**Independent Test**: 单测验证重复 Start 幂等；日志验证清理阶段先于设备创建执行

### Implementation for User Story 1

- [X] T003 [US1] `Manager.Start` 前置 `cleanupResiduals` 显式阶段：等待上次释放完成（现有 `waitReleasingLocked` 12s 上限维持）→ 若仍有残留资源执行一次 `releaseLocked` → 输出阶段耗时日志；整体仍在现有 defer 回滚保护内（`pkg/singtun/manager.go`）
- [X] T004 [P] [US1] 残留网卡清理从 `newTunWithRetry` 内部提升至清理阶段：`cleanupStaleAdapters` 返回移除数量并记入启动日志，`adapter_other.go` 保持 no-op 对齐签名（`pkg/singtun/adapter_windows.go`、`pkg/singtun/adapter_other.go`）
- [X] T005 [US1] 生命周期单测：重复 Start 幂等（running 时直接成功）、清理阶段先于 tun.New 执行的顺序断言（可注入 fake tun 工厂或以日志顺序断言）（`pkg/singtun/lifecycle_test.go`）

**Checkpoint**: 强杀进程后重启，日志可见"残留清理"阶段及移除数量，启动成功

---

## Phase 4: User Story 2 - 启停稳定（内存有界、不冻结） (Priority: P0) 🎯 MVP

**Goal**: 关闭路径全部有界，冻结根因消除，内存问题经 instrumentation 定位
**Independent Test**: 单测注入挂死的 fake Close 验证 Stop 有界返回；启停循环观察 RSS 与连接计数日志

### Implementation for User Story 2

- [X] T006 [US2] `closeWithTimeout` 硬上限：超时后再等硬上限（30s），到顶记录泄漏告警并继续后续清理（不再无限期 `<-done`）；中文注释说明"残留设备由下次启动 cleanupResiduals 兜底"的设计转移（`pkg/singtun/manager.go`）
- [X] T007 [US2] `Handler.Release` 收敛上限：快照循环最多 5 轮，每轮记录剩余连接数，到顶记告警返回（`pkg/singtun/handler.go`）
- [X] T008 [US2] 启停 instrumentation：各阶段边界记录进程 RSS 与 Handler 存活连接数/包连接数（debug 级），供定位数十 GB 尖峰（`pkg/singtun/manager.go`、`pkg/singtun/handler.go`）
- [X] T009 [US2] 有界关闭单测：以可注入的挂死 Close 验证 Stop 在上限内返回且继续执行后续清理；Stop/Shutdown 幂等（连续调用不报错不重复清理）（`pkg/singtun/lifecycle_test.go`）

**Checkpoint**: 连续启停 ≥20 次，RSS 无单调增长，单次启停无 GB 级尖峰，全程 UI 可响应

---

## Phase 5: User Story 3 - SNI 解析模块化（ECH/分片/畸形） (Priority: P1)

**Goal**: 独立 SNI 解析模块，已读字节经包装连接完整回放
**Independent Test**: 纯函数单测注入构造字节流（标准/分片/畸形×N/ECH/非TLS），无需真实 TUN

### Implementation for User Story 3

- [X] T010 [P] [US3] 新增解析模块：`ParseClientHelloRecord(record []byte) (SNIInfo, error)` 纯函数（SNI 提取、ECH 扩展检测、全部边界长度校验、畸形返回 Err 不 panic）+ `SniffClientHello(conn, timeout) (SNIInfo, net.Conn)`（bufio 包装、超时恢复 deadline、返回连接保证回放）（`pkg/singtun/sni_parser.go`）
- [X] T011 [P] [US3] 解析单测：标准 ClientHello、record 头与 body 分片到达、非握手记录、截断 body、长度字段越界、空 server_name 列表、含 ECH 扩展（外层 SNI + HasECH）、非 TLS 首字节；全部断言不 panic（`pkg/singtun/sni_parser_test.go`）
- [X] T012 [US3] Handler 切换到新解析模块：替换内联 `sniffSNIFromReader`，行为兼容（超时/非 TLS 返回空 SNI + 包装连接）（`pkg/singtun/handler.go`）

**Checkpoint**: `go test ./pkg/singtun/ -run SNI` 全绿，Handler 嗅探行为与切换前一致

---

## Phase 6: User Story 4 - 规则匹配 + SNI 重写透传 (Priority: P1)

**Goal**: 命中规则且 SniFake 非空 → TUN 层重写 ClientHello SNI 后透传；未命中字节级透传
**Independent Test**: 重写纯函数单测（自洽校验：重写后再解析 SNI == 新值）；决策方法单测（含 ~ 正则与非法正则跳过）

### Implementation for User Story 4

- [ ] T013 [US4] 重写模块：`RewriteClientHelloSNI(record []byte, newSNI string) ([]byte, error)`——定位 server_name 扩展、替换名称、重算 server_name 扩展内层长度/扩展列表长度/握手长度/记录长度；检测到 ECH 返回明确错误（调用方跳过重写）；非法输入返回错误不 panic（`pkg/singtun/sni_rewrite.go`）
- [ ] T014 [P] [US4] 重写单测：等长替换、变长（变短/变长）、各层长度重算正确性、ECH record 拒绝、自洽校验（重写结果再次解析 SNI == 新值且结构完整）（`pkg/singtun/sni_rewrite_test.go`）
- [ ] T015 [P] [US4] ProxyServer 决策方法：`SNIRewriteDecisionForTUN(host string) (rewriteTo string, matched bool)`——包装 `matchRule` 并提取 `SniFake`；补充 `~` 正则匹配与非法正则跳过的单测（`proxy/proxy.go`，测试入 `proxy/rules_sni_decision_test.go`）
- [ ] T016 [US4] Handler 集成：`SetSNIDecider(fn func(sni string) (rewriteTo string, matched bool))` 注入；惰性嗅探（目标为 IP 或已知域名规则含 SniFake 时才读首包）；重写经 `prefixConn`（前缀字节先于底层连接数据）注入转发路径；ECH 流量跳过重写仅记录决策日志；回调为 nil 时行为等同现状（`pkg/singtun/handler.go`）
- [ ] T017 [US4] 装配点注入：`core_runtime.go` 在创建 nativeTUN 后经 Manager/Handler 注入 `proxyServer.SNIRewriteDecisionForTUN` 回调（`core/core_runtime.go`）

**Checkpoint**: 本地代理端收到命中规则流量的 ClientHello SNI 为 SniFake 值；未命中流量与原始流量逐字节一致（单测断言 prefixConn 行为）

---

## Phase 7: User Story 5 - 日志分级与可观测性 (Priority: P2)

**Goal**: 关键路径分级日志完备，启停问题可凭日志定位
**Independent Test**: 启停 + 触发一次规则命中，检查各级别日志条目存在

### Implementation for User Story 5

- [ ] T018 [US5] 日志条目补全：启停各阶段（含耗时）、残留清理移除数量、SNI 嗅探结果（含 ECH 标记）、重写决策（原 SNI → 新 SNI）、有界关闭的超时/泄漏告警，全部经 slog 分级输出；确认不输出完整域名以外的敏感内容（`pkg/singtun/manager.go`、`pkg/singtun/handler.go`）

**Checkpoint**: 各级别日志条目可凭日志复盘一次完整启停 + 重写决策

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: 全量回归与代码质量收尾

- [ ] T019 全量回归与质量检查：`go vet ./pkg/singtun/... ./proxy/... ./core/...`、`go build -tags with_gvisor ./...`、`go test ./pkg/... ./proxy/...`；确认关键设计决策有中文注释（残留清理兜底关系、ECH 不重写理由、惰性嗅探条件）；无调试残留（仓库根目录）
- [ ] T020 前端类型回归：`cd frontend && ./node_modules/.bin/tsc --noEmit`（确认后端签名未变故前端零影响；若失败仅限与本特性无关的既有问题则记录说明）（`frontend/`）

---

## Phase 9: Verification

<!-- verification_scope: build-only -->

**Purpose**: 构建 + 测试 + 后端整体构建验证（Go 后端模块，无 UI 验证任务）

- [ ] T021 构建验证：`go build ./...` 通过；出现编译错误则修复后重跑直至通过（仓库根目录）
- [ ] T022 测试验证：`go test ./pkg/... ./proxy/...` 全部通过；失败用例修复后重跑（仓库根目录）
- [ ] T023 后端整体构建验证：`.\build_windows.ps1 -Build backend -Silent` 通过（含 `-BuildMsix` 不在本阶段范围）（仓库根目录）

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1/US2 同改 manager.go/handler.go，建议按优先级串行（P0 先行）
  - US3 可与 US1/US2 并行（新文件为主）；US4 依赖 US3 完成后集成
- **Polish (Phase 8)**: Depends on all user stories being complete
- **Verification (Phase 9)**: Depends on Polish completion

### User Story Dependencies

- **User Story 1 (P0)**: 可在 Foundational 后立即开始 - MVP 核心
- **User Story 2 (P0)**: 可在 Foundational 后开始 - 与 US1 共享文件，串行提交避免冲突
- **User Story 3 (P1)**: 可与 US1/US2 并行（新文件 sni_parser*）；T012 触及 handler.go 需与 US2 协调
- **User Story 4 (P1)**: T016 汇聚依赖 T012（解析）+ T013（重写）+ T015（决策）；T017 依赖 T016
- **User Story 5 (P2)**: 依赖 US2/US4 的改动点完成后统一补日志

### Within Each User Story

- Tests MUST be written and FAIL before implementation（T011/T014 尤其遵循）
- 纯函数模块（parser/rewrite）先于 Handler 集成
- Handler 集成先于 core 装配
- Story complete before moving to next priority

### Parallel Opportunities

- T004（adapter_windows.go）与 T003/T005（manager.go/lifecycle_test.go）不同文件可并行
- T010/T011（sni_parser 新文件）与 US1/US2 的 manager.go 改动可并行
- T014（sni_rewrite_test.go）与 T015（proxy.go）不同文件可并行
- 全部 [P] 标记任务见各 Phase

## Parallel Example: User Story 4

```bash
# Launch independent US4 tasks together:
Task: "T014 [P] [US4] 重写单测 in pkg/singtun/sni_rewrite_test.go"
Task: "T015 [P] [US4] ProxyServer 决策方法 in proxy/proxy.go"

# Then converge:
Task: "T016 [US4] Handler 集成 in pkg/singtun/handler.go (depends on T012, T013, T015)"
```

## Implementation Strategy

### MVP First (User Story 1 + 2 Only)

1. Complete Phase 1: Setup（基线确认）
2. Complete Phase 2: Foundational（slog 桥接）
3. Complete Phase 3 + 4: US1 残留清理 + US2 有界关闭
4. **STOP and VALIDATE**: 启停循环实测（内存/冻结/残留清理日志）
5. P0 痛点闭环即达成 MVP

### Incremental Delivery

1. Setup + Foundational → 日志桥接就绪
2. + US1/US2 → 稳定性 MVP（P0 交付）
3. + US3 → SNI 解析模块化（可独立验证）
4. + US4 → SNI 重写能力（依赖 US3）
5. + US5 → 可观测性收尾 → Polish → Verification

## Notes

- **总任务数**: 23（T001–T023）；US1×3、US2×4、US3×3、US4×5、US5×1，基础/收尾/验证×7
- **MVP 范围**: T001–T009（残留清理 + 有界关闭 = 用户痛点最小闭环）
- 所有修改保持对外签名兼容（NewManager/Start/Stop/Shutdown/Status 不变），App 层零改动
- 内存尖峰根因修复以 T008 instrumentation 的实测数据为准，避免猜测性重构
- [P] tasks = different files, no dependencies
- Commit after each task or logical group（提交信息遵循仓库规范：英文 conventional-commit）
