# Per-Application Routing

A site group's "Per-Application Routing" field restricts a rule to connections initiated by the listed processes (e.g. `telegram.exe, discord.exe`). This document describes platform support, how application identity is detected, and how application rules interact with domain rules and default routing.

## Platform support and limitations

| Platform | Supported | Detection |
|----------|-----------|-----------|
| Windows | ✅ | TCP table (`GetExtendedTcpTable`) → PID → process name, cached |
| Linux | ✅ | `/proc/net/tcp*` inode → `/proc/<pid>/comm` |
| macOS | ✅ | `lsof` connection lookup → PID → process name (first call has a cold-start cost, cached afterwards) |
| Other platforms | ❌ | The process is always unknown; process rules never match (fail closed) |

- The process lookup only runs when **enabled process rules exist**; with none present the cost is zero.
- Only the **executable base name** matches (no path), comma separated, case-insensitive.
- When the process is unknown (lookup failure, system restrictions, unsupported platform) the process rule never applies — other traffic is never affected by mistake.
- Loading a configuration that contains process rules on an unsupported platform produces a platform warning in the migration report.

## Interaction with domain rules / default routing

1. **Process rules outrank every domain rule**: when a target matches both a process rule and a domain rule, the process rule wins (the process bonus outranks all domain score classes).
2. **Process + domain rules require both to match**: the domain field narrows the process rule; a domain miss takes the rule out of contention (it does not fall back to a process-only rule).
3. **Empty domain list = all destinations for that process**: a process rule with an empty (or `*`) domain list covers every connection of that process.
4. **Nothing matches → default routing**: with no rule matched, the global mode and auto-routing make the default decision.
5. The rule match explanation tool (Rules page "Test") accepts a process name and simulates process-rule matching through exactly the same pipeline as real connections.

Conflict regression cases live in `app/app_perapp_test.go` (process vs domain, process+domain, unknown process, default routing).
