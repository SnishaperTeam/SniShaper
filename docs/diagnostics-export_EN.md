# Diagnostic Export

The "Diagnostics" button on the Logs page exports a zip for bug reports in one click. A preview dialog lists the included files and the redacted fields before anything is written. Exporting only reads runtime state and config files — a failure never affects the running proxy.

## Package Format

The zip is written next to the application logs, named `snishaper-diagnostics-<timestamp>.zip`, and contains:

| File | Contents |
|---|---|
| `diagnostics.json` | export time, version, platform, service/proxy/system-proxy/TUN state, self-check result, file manifest |
| `settings.json` | **sanitized copy** of `config/settings.json` |
| `rules.json` | **sanitized copy** of `rules/config.json` |
| `logs.txt` | the last 500 in-memory application log lines (sanitized) |

Top-level shape of `diagnostics.json`:

```json
{
  "generated_at": "RFC3339 timestamp",
  "version": "app version",
  "diagnostics": { "version": "...", "platform": "...", "system_proxy": {...}, "tun": {...} },
  "self_check": { "ok": true, "checks": [ { "name": "service", "ok": true } ] },
  "files": [ { "name": "settings.json", "size": 123, "description": "..." } ]
}
```

## Redaction Rules

Before export, the config copies and log text are sanitized as follows (source files are never modified):

- **Sensitive keys**: string values under keys matching `api_key` / `apikey` / `token` / `secret` / `password` / `authorization` (case-insensitive, substring match) are replaced with `[REDACTED]`.
- **URL credentials**: userinfo in `scheme://user:pass@host` is replaced with `[REDACTED]`.
- **Query parameter tokens**: values of `?api_key=…&token=…&secret=…&password=…` are replaced with `[REDACTED]`.
- **Bearer tokens**: the token part of `Bearer <token>` is replaced with `[REDACTED]`.
- Unparseable config files are included as text after the same textual redaction (surfacing the problem instead of silently dropping it).

## Known Limitations

- Redaction is field-name and pattern based; **review the zip yourself** before sharing it to confirm it contains nothing else sensitive.
- Domains, rules and IP lists are diagnostic essentials and are not redacted.
