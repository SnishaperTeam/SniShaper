# Configuration Profiles

A configuration profile is a **named snapshot of the configuration** (one `settings.json` + one `rules/config.json`) saved for a specific network or usage scenario. This document covers the storage layout and export format for users and developers who need them; day-to-day usage is described in the "GUI Configuration" wiki page.

## Storage layout

```
<user config dir>/         # Windows: %APPDATA%\snishaper, Linux/XDG: ~/.config/snishaper
└── profiles/
    ├── _active.json       # active-profile marker {"name": "...", "switched_at": "..."}
    ├── work/
    │   ├── settings.json  # this profile's settings snapshot
    │   └── rules.json     # this profile's rules snapshot
    └── home/
        ├── ...
```

- **The live files are always the single runtime source of truth**: `config/settings.json` and `rules/config.json` are never moved; profiles only copy into and out of them on switch. Installations that never use profiles are completely unaffected (existing configurations are preserved by construction).
- Each profile directory holds its own complete rules (including per-site-group DNS modes), upstreams, port and other settings — isolated by construction.
- Name rules: letters, digits, CJK characters, spaces, `_` and `-`, up to 64 chars; a leading `_` is reserved.

## Switch semantics (safe behavior with active connections)

1. If the proxy is running: stop it first (TUN / system proxy / active connections go down with it).
2. Snapshot the current live config back into the **outgoing** profile directory (so unsaved edits survive the switch).
3. The target profile's two snapshots replace the live files.
4. The in-app rule manager and the core subprocess each reload; the listen port follows the target profile.
5. **The proxy is never restarted automatically**: the user starts it manually after the switch (the UI states this in the confirmation dialog).

## Export / import format

An export is a single JSON file (written under the log directory's `profile-exports/`):

```json
{
  "format": "snishaper-profile",
  "version": 1,
  "name": "work",
  "exported_at": "2026-10-30T12:00:00Z",
  "settings": { ... },
  "rules": { ... }
}
```

Before committing an import, the `settings` / `rules` payloads are written to a temp directory and validated through the **same RuleManager load pipeline as live matching**: future config versions, unparseable JSON and non-object structures are rejected with the engine's own explanation, and migration warnings are passed through to the UI verbatim. A failed validation never writes any profile.
