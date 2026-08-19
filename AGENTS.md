# Build & Test

- Build all: `.\build_windows.ps1 -Build all -Silent`
- Build backend only: `.\build_windows.ps1 -Build backend -Silent`
- Build frontend only: `.\build_windows.ps1 -Build frontend -Silent`
- Build + MSIX: `.\build_windows.ps1 -Build all -Silent -BuildMsix`
- Frontend dev: `cd frontend && npm run dev`
- Frontend install: `cd frontend && npm install`
- Go mod tidy: `go mod tidy`
- Go build check: `go build -tags with_gvisor ./...`
- Frontend type check: `cd frontend && ./node_modules/.bin/tsc --noEmit` (authoritative; `vite build` does NOT run strict type checks, so it can miss TS errors)
- No test framework exists (no test files in repo)

# Project Overview

SniShaper is a Windows local proxy tool using ECH Injection, TLS-RF Fragmentation, QUIC Connection Rebuilding, and Lightweight Server Mode Relay. Frontend: React 19 + Wails v3 + MUI + Vite 8. Backend: Go 1.25+ with Wails v3 bindings.

# Architecture

- `/main.go` — entry point, Wails app + tray setup
- `/app/` — Go backend: App struct, API bindings (`app_api.go`), system proxy (`app_system.go`), autostart
- `/core/` — Mihomo core process management (`core_client.go`), TUN, admin elevation
- `/proxy/` — Proxy server: MITM, SOCKS5, TLS fragmentation, TUN flow, auto-routing, rules
- `/pkg/rules/gfwlist.go` — GFWList domain list: local file/reader loading + `IsBlocked` suffix match. NOTE: online update (fetch from URL, cache write) was removed; only local `gfwlist_cache.txt` (next to rules file) is loaded at startup, no network calls
- `/pkg/` — Packages: cert manager, CF IP pool, DoH resolver, sysproxy, TLS fragmentation
- `/evolution/` — Rule evolution testing: tester, rule generator, result analysis
- `/frontend/src/` — React SPA: 10 pages (Dashboard, Proxies, Rules, Routing, DNS, Evolution, Logs, Settings, About, Welcome)
- `/rules/config.json` — Site group rules (~3800 lines), MITM/ECH/SNI-fake configs
- `/config/settings.json` — App settings (port, TUN, theme, Cloudflare IPs)

Data flow: Frontend (React HashRouter) → Wails bindings (`EventsOn` + async Go calls) → App API → proxy/core subsystems. Frontend state sync via `app:state_changed` events from `emitFrontendState()`.

# Conventions

- Go: standard layout, `snishaper` module path, no external DI framework
- Frontend: React 19 + TypeScript 6, `@/` path alias for `./src/`, MUI v9 + emotion for styling, `@heroicons/react` for icons
- Error handling: log via `a.appendLog()`, return errors to frontend; no panic except top-level recover in `main.go`
- Naming: Go `camelCase` for unexported, `PascalCase` for exported; TS `camelCase` for functions/vars, `PascalCase` for components/types
- Imports: Go stdlib → third-party → internal; TS: React → components → API → i18n
- i18n: `I18nContext` provider, `useTranslation()` hook, lazy page loading with `Suspense`
- i18n `t(path, params)` resolves dotted paths against zh/en/ru; a **missing key returns the key itself (truthy)** — so `t(key) || 'fallback'` is dead code and must NOT be written; fallback pattern is `label === key ? fallback : label`
- i18n dynamic keys (`t(\`section.${x}\`)`) MUST have a missing-key guard, otherwise unknown values render as the raw key
- i18n placeholders: param names passed to `t('k', {...})` must match `{param}` placeholders in all three locales (shorthand props like `{ ms }` count as params); keep the key tree structurally identical across zh/en/ru
- i18n coverage: aria-label / placeholder / title / toast / button text must go through `t()`; add new UI copy to ALL of zh/en/ru JSON in the same commit
- Git: commit messages MUST be written in English (user requirement), conventional-commit style, e.g. `refactor: remove GFWList dynamic update and complete i18n`

# Security & Environment

- Dev certificate: `devcert.pfx` (password: `password`)
- Cert manager generates CA at `pkg/certmanager/` (runtime, gitignored)
- Admin rights required for system proxy registry writes and TUN
- Registry access via `golang.org/x/sys/windows/registry` (not `reg.exe`)
- MSIX packaging requires PFX signing cert; CI uses self-signed

# Evidence Required

- Changes to `app_system.go`, `app_api.go` — verify proxy start/stop + system proxy toggling
- Changes to `rules/config.json` — verify correct site group, MITM/ECH modes
- Changes to frontend event handling — verify `app:state` / `app:state_changed` events
- Changes to core client — verify TUN ready + wintun.dll presence
- Changes to i18n keys/text — verify `tsc --noEmit` passes and zh/en/ru key trees stay consistent (no missing keys, no placeholder mismatches)
