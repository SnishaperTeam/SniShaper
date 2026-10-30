# Network Regression Test Suite

Scope, execution and limits of the network regression tests in this repository. Goal: automate every network-state regression case that runs reliably without elevation or real TUN hardware, and document the rest.

## Automated cases (no elevation required, run locally)

CI is build-and-smoke only and does not run the tests; developers run the following locally (see the verification commands in `AGENTS.md`):

```pwsh
go test -race -tags with_gvisor ./...
cd frontend && npx tsc --noEmit
```

Coverage:

| Case | Location | Notes |
| --- | --- | --- |
| System proxy restoration state machine (repeated cycles) | `app/app_regression_test.go` | 20 rounds of enable -> crash -> recover -> clean shutdown; marker bookkeeping must not regress across port changes |
| Recovery from partial setup | same | Leftover marker + system proxy already off (system restart scenario) -> marker cleanup only, disable is never called |
| Foreign proxy is never overwritten | same | Marker exists but the system proxy points elsewhere -> report only, no modification |
| Permission failure retries | same | Disable fails (simulated missing admin rights) -> error surfaces, marker survives, next startup retries |
| Concurrent decisions (-race) | same | 64 goroutines hammering the pure decision core |
| Crash / interrupted shutdown / corrupt marker decision paths | `app/app_recovery_test.go` | Table-driven: forced termination, failed startup, interrupted shutdown, system restart |
| Diagnostic export and redaction | `app/app_diag_export_test.go` | Sensitive-field redaction; export failures never affect the running proxy |
| Self-update install, autostart, rules update | `app/*_test.go`, `internal/rulesguard`, `common`, `core` | Per-module regressions |

Platform effects are simulated through the `sysproxyStatusFn` / `sysproxyDisableFn` seams (`app/app_system.go`): tests never mutate the real system proxy settings of the machine they run on and only touch marker files inside `t.TempDir()`.

## Cases that need elevation or real hardware (manual)

These do NOT run in CI:

| Case | Why | Manual procedure |
| --- | --- | --- |
| TUN startup/shutdown/restart and repeated cycles, route installation and cleanup | Needs admin rights plus the wintun driver (Windows) or `/dev/net/tun` (Linux) | Run the GUI/CLI as admin, toggle `tun on` / `tun off` 10+ times, watch `tun status` and the routing table |
| Live DNS resolution, IPv4/IPv6 behavior | Depends on internet connectivity and ISP environment | With the proxy on, `curl -k -m 15` IPv4-only and IPv6-capable sites; for the headless Linux/WSL build see the TUN test flow in `AGENTS.md` |
| Real system proxy registry writes and restoration | Needs admin rights and mutates the test machine's network | Run the GUI as admin, toggle the system proxy switch repeatedly, verify the registry is restored on exit |
| Long-running sessions and resource usage | Infeasible CI duration | Run locally for 24h+; watch memory (the connection registry is bounded at 4096 entries) and CPU |

## Network tests in the core library (snishaper-core)

Engine-level tests live in the `github.com/SnishaperTeam/snishaper-core` repository; `go test ./...` runs them (no elevation required):

- `proxy/proxy_integration_test.go` — HTTP proxy end-to-end, Start/Stop idempotency
- `proxy/connections_test.go` — connection registry: register/snapshot/remove, bounded limit, byte counters
- `proxy/dnsmode_test.go` — DNS mode decisions
- `proxy/tun_config_test.go` — TUN config validation
- `proxy/config_migration_test.go` — configuration migration compatibility
 - `proxy/bench_test.go` — performance benchmarks (see `docs/benchmarks_EN.md`)
- `netiface`, `singtun`, `sysproxy` — unit tests for interface enumeration, TUN, and per-platform system proxy implementations
