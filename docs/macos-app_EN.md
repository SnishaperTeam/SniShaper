# macOS Menu Bar App (SnishaperBar)

On macOS, SniShaper ships as a **menu bar app**: the Swift package under `macos/` (SnishaperBar) renders status and controls, while the proxy engine itself lives in the headless `snishaper` CLI binary bundled inside the app.

## Architecture

```
SniShaper.app
├── Contents/MacOS/SnishaperBar            # menu bar app (Swift, built with SPM)
├── Contents/Resources/bin/snishaper       # headless CLI (Go, the proxy engine)
└── Contents/Info.plist                    # LSUIElement=true: menu bar only, no Dock icon
```

- The menu bar app **implements no proxy logic**: start/stop service, proxy and system-proxy toggles all shell out to CLI subcommands (`snishaper start/stop/proxy on/off/sysproxy on/off/diag`).
- Status polling uses the stable JSON contract of `snishaper status --json` (every 3 seconds); human-readable text is never parsed.
- TUN, rules, subscriptions, ECH and everything else come from the CLI process; see `snishaper help` for command usage.

## Building

Requirements: macOS 13+, Xcode Command Line Tools (swift 5.9+), Go 1.27+.

```bash
# One-shot bundle (builds the bar app + headless CLI, assembles the .app and a zip)
macos/scripts/bundle-app.sh

# Bar app only
cd macos && swift build -c release
```

Output: `build/bin/macos/SniShaper.app` and `SniShaper-macos-<arch>.zip`.

CI builds and bundles on native Apple Silicon and Intel macOS runners (`.github/workflows/macos.yml`).

## Installing

1. Download `SniShaper-macos-<arch>.zip` and unzip it.
2. Drag `SniShaper.app` into `/Applications` (or `~/Applications`).
3. First launch: **builds are unsigned/un-notarized**, so Gatekeeper blocks them. Allow it one of these ways:
   - Right-click `SniShaper.app` → "Open" → confirm (recommended — allows only this app);
   - or System Settings → Privacy & Security → the "was blocked" notice at the bottom → "Open Anyway".
4. The ⬢ icon in the menu bar means it is running.

## Upgrading

1. Quit the menu bar app (menu → Quit SniShaper).
2. If the service is running, stop it first (quitting the bar app does not stop the standalone service process): `Contents/Resources/bin/snishaper stop`.
3. Replace the old `SniShaper.app` with the new one.
4. Reopen the app.

Config and logs live next to the executable per the CLI conventions, so upgrades **never delete** user data.

## Uninstalling

1. Quit the menu bar app and stop the service (see above).
2. Delete `SniShaper.app`.
3. Disable the system proxy manually if it is still on: System Settings → Network → Proxies.
4. Optional: remove the config and log directories the CLI created (`config/`, `rules/`, `log/` next to the binary).

## Limitations

- **Unsigned/un-notarized**: Gatekeeper must be re-allowed after every upgrade.
- The service process (CLI) needs administrator privileges for TUN and system-proxy changes; on macOS the system proxy is applied via `networksetup`, TUN uses utun, both may trigger authorization prompts.
- The bar app exposes the core controls (service, proxy, system proxy, diagnostics, logs); use the CLI alongside it for the full feature set.
- See [Platform_EN.md](Platform_EN.md) for platform support details.
