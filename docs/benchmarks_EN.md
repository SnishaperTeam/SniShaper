# Proxy Performance Benchmarks

The repeatable performance benchmarks live in `proxy/bench_test.go` inside the core library `github.com/SnishaperTeam/snishaper-core` (referenced directly through the go.mod pin in this repository — no separate clone needed). This document describes how to run them, the workloads they cover and the statistics methodology; the goal is to validate performance changes through benchmark comparisons, **never through a single run**.

## Running

```pwsh
# All benchmarks (-count 10 produces benchstat comparison samples; -benchmem is mandatory:
# allocations are the primary signal of data-plane regressions)
go test -run '^$' -bench 'Benchmark' -benchmem -count 10 github.com/SnishaperTeam/snishaper-core/proxy/ > new.txt

# Quick smoke (for dev iteration; results are NOT comparison material)
go test -run '^$' -bench 'Benchmark' -benchmem -benchtime 100x github.com/SnishaperTeam/snishaper-core/proxy/
```

## Covered proxy modes and network paths

Both entry points — HTTP CONNECT and SOCKS5 — and the full direct-mode path (routing → dial → tunnel forwarding). MITM/TLS modes depend on certificates and the TLS stack; when touching those paths use the direct results as the baseline and spot-check manually in a real environment.

## Representative workloads

| Benchmark | Workload |
|-----------|----------|
| `BenchmarkTunnelThroughput` | Sustained transfers (large downloads/uploads): 64 KiB round trips |
| `BenchmarkTunnelShortConnections` | Short-connection heavy requests (API-intensive): tunnel setup + small payload + teardown |
| `BenchmarkTunnelConcurrent` | Many concurrent connections: `RunParallel` concurrent tunnels with small round trips |
| `BenchmarkSOCKS5ShortConnections` | Short-connection churn for SOCKS5 clients |
| `BenchmarkRuleMatch/rules=100\|1000` | Per-request rule decision cost (light/heavy rule sets, hot path) |
| `BenchmarkRuleMatchProcess` / `BenchmarkFindProcessName` / `BenchmarkProcessNameInList` | Per-app routing decisions and OS-level process resolution |
| `BenchmarkNormalizeHost` | Per-request host normalization |
| `BenchmarkConnRegistryRegisterRemove` / `BenchmarkConnRegistrySnapshot` | Connection tracking register/remove and the connection viewer snapshot (registry at capacity, 4096 entries) |

Throughput and latency figures come from the benchmark timings (ns/op; B/op and allocs/op are the allocation figures; CPU time vs wall clock can be profiled further with `-cpuprofile`).

## Statistics (cross-release comparison)

Single runs are heavily affected by machine load, temperature and power policy. Comparison workflow:

```pwsh
go install golang.org/x/perf/cmd/benchstat@latest

# Baseline (before the change) → old.txt; after the change → new.txt (commands above, -count 10)
benchstat old.txt new.txt
```

Acceptance criteria:

- A p-value ≥ 0.05 means **no significant difference** — neither a regression nor an improvement.
- On a significant change (p < 0.05) larger than 5%, check whether the allocation count (allocs/op) moved in the same direction before drawing conclusions.
- Performance-related PRs attach the benchstat output; results that cannot be reproduced are not merge criteria.

## Environment recording

Benchmark output carries `goos / goarch / cpu` fields; save them together with the results. Both sides of a comparison must satisfy:

- The same physical machine (or the same CI runner model), the same power policy.
- The same Go version (`go version`).
- No other heavy background tasks.

CI does not run benchmarks (shared runners are too noisy for comparable results); comparisons are done on developer machines or dedicated hardware.

## Known limits

- Throughput/latency benchmarks run over loopback: they measure the **relative data-plane overhead** (buffering, tracking, locks), not absolute WAN bandwidth.
- `BenchmarkFindProcessName` on macOS includes the lsof cold start in the first iteration; ignore the first sample in benchstat or pin the iteration count with `-benchtime`.
- Benchmarks run against the core version pinned in go.mod; re-baseline after upgrading core.
