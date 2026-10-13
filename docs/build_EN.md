# SniShaper Build Guide

[中文](build.md) | [English](build_EN.md) | [Русский](build_RU.md)

> Back to [README](README_EN.md)

This project is built with **Wails v3 + React 19 + MUI**, with a **Go** backend. `build.sh` (Linux / macOS / WSL) and `build_windows.ps1` (Windows) share one target matrix and output layout.

## Table of Contents

- [Artifact matrix (12 targets)](#artifact-matrix-12-targets)
- [Windows Build](#windows-build)
- [Linux Build](#linux-build)
- [Version & Release Channel](#version--release-channel)
- [Development Environment](#development-environment)
- [Continuous Integration](#continuous-integration)
- [Cross-Platform Notes](#cross-platform-notes)

---

## Artifact matrix (12 targets)

| Type | Platform | Arch | Output |
| --- | --- | --- | --- |
| CLI | Windows | `x64` / `x86` / `arm64` | `build/bin/cli/Windows/<arch>/snishaper.exe` |
| CLI | Linux | `x64` / `arm64` | `build/bin/cli/Linux/<arch>/snishaper` |
| CLI | Darwin | `x64` / `arm64` | `build/bin/cli/Darwin/<arch>/snishaper` |
| GUI | Windows | `x64` / `x86` / `arm64` | `build/bin/gui/Windows/<arch>/snishaper.exe` |
| GUI | Linux | `x64` / `arm64` | `build/bin/gui/Linux/<arch>/SniShaper` |

7 CLI + 5 GUI = **12 targets**; every target directory also carries the `config/` and `rules/` seed folders. GUI is never built for Darwin, and `x86` exists on Windows only. The GUI needs GTK/WebKit (Linux) and the frontend build, so it only builds on a same-OS native host; the CLI is pure Go (`CGO_ENABLED=0`) and all seven targets can be emitted from one runner without any cross toolchain.

### Flags

Flags (`build.sh` / `build_windows.ps1`): `--platform` / `-Platform`, `--arch` / `-Arch`, `--type` / `-Type` (repeatable; PowerShell uses the comma-array form, e.g. `-Type cli,gui`), `--all` / `-All`, `--dry-run` / `-DryRun`, `--ci` / `-CI` (no prompts, never exports cross `CC`/`CXX`), `--cross` / `-Cross` (local only), `--install-deps` / `-InstallDeps`, `--gtk3` / `-Gtk3`, `--wails` / `-Wails`, `--silent` / `-Silent`, `--help` / `-Help`.

### Examples

```bash
# Linux / macOS / WSL
./build.sh --all                                            # all 12 targets
./build.sh --type cli --all                                 # all 7 CLI targets
./build.sh --platform windows --arch arm64 --type cli --type gui
./build.sh --ci --platform linux --arch arm64 --type cli --type gui
./build.sh --dry-run --all                                  # plan only
```

```powershell
# Windows
.\build_windows.ps1 -All
.\build_windows.ps1 -Type cli -All
.\build_windows.ps1 -CI -Platform windows -Arch arm64 -Type cli,gui
.\build_windows.ps1 -Platform windows -Arch x64 -Type gui -InstallDeps -BuildMsix
.\build_windows.ps1 -DryRun -All
```

### CI: ARM64 is built natively

The regular CI workflow (build.yml, push/PR) only builds and smoke-tests - no packaging, no artifact uploads. The GUI matrix runs one (platform, arch) pair per job, so the ARM64 GUI is only compiled on native ARM runners: `linux/arm64` on `ubuntu-24.04-arm`, `linux/x64` on `ubuntu-latest`, and `windows/x64|x86|arm64` on `windows-latest` (Go emits `windows/arm64` with `CGO_ENABLED=0`; no cross toolchain). All seven CLI targets are pure Go (`CGO_ENABLED=0`) and are built once by the single `cli-build` job, so nothing is built twice. Packaging (MSIX / 7z / tar.gz) happens only in the release pipeline (tag or manual dispatch). Under `--ci` the scripts never export `CC`/`CXX`, so ARM64 logs cannot contain `aarch64-linux-gnu-gcc` or `osxcross`; the CI job greps the log and fails if either appears. For the Windows GUI version resource, it is regenerated per target architecture right before linking with go-winres (`--arch amd64/386/arm64`, emitted as `rsrc_windows_<arch>.syso`); no bare `.syso` is kept in the repository, so Linux/Darwin builds never link a Windows resource object.

### Interactive mode

Running with no selection flags opens the interactive wizard (language, type, platform, arch, optional cross toolchain, then confirmation of the plan).

### Artifact verification

Verify artifacts with `file build/bin/gui/Linux/arm64/SniShaper` (ELF aarch64) or, on Windows, `dumpbin /headers ...` / the PE machine field (`0x8664` = x64, `0xAA64` = arm64, `0x014C` = x86).

---

## Windows Build

```powershell
# Clone the repository
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# Full compilation (interactive mode, auto-installs deps, optional MSIX)
powershell -ExecutionPolicy Bypass -File .\build_windows.ps1

# Or with PowerShell 7
pwsh -ExecutionPolicy Bypass -File .\build_windows.ps1
```

### Build Script Command-Line Parameters

`build_windows.ps1` supports the following parameters to skip interactive prompts:

| Parameter | Values | Description |
| ------------- | ------------------------------ | ---------------------------------------------------------------- |
| `-Build` | `<system> <mode> <scope>` | Three-part build spec. **System**: `windows` / `linux` / `all`; **Mode**: `gui` / `cli` / `all`; **Scope**: `frontend` / `backend` / `all`. Omit for interactive menu; legacy `-Build frontend/backend/all` still works |
| `-Lang` | `en` / `cn` / `ru` | Prompt language, defaults to English |
| `-Arch` | `x64` / `arm64` / `x86` | Target architecture (accepts amd64/386 aliases), defaults to host. `x86` only builds Windows — Linux (incl. CLI) and Darwin are skipped |
| `-InstallDeps` | No value (switch) | Run `npm install` before the frontend build |
| `-BuildMsix` | No value (switch) | Build an MSIX installer after compilation (requires WinApp CLI) |
| `-SkipSign` | No value (switch) | Skip MSIX signing, output file gets `unsigned_` prefix (requires `-BuildMsix`) |
| `-Cli` | No value (switch) | Additionally build the headless CLI (target platforms determined by `-Arch`) into `build/bin/cli/<Platform>/<Arch>/` |
| `-Gtk3` | No value (switch) | Use GTK3 + webkit2gtk-4.1 for Linux (WSL) builds (equivalent to `build.sh --gtk3`) |
| `-Silent` | No value (switch) | Silent mode, skip all interactive prompts; defaults to `-Build windows gui all` and `-Lang en` when omitted |

**Behavior notes:**

- **Auto-elevation**: The script requires administrator privileges. When run as a regular user, it relaunches itself via a UAC prompt and passes all parameters through unchanged.
- **Pre-build cleanup**: Any running `snishaper` processes are force-terminated before the build to avoid file locks.
- **Version sync**: Before compiling the backend, the version and release channel are read from `Package.appxmanifest`, synced into the version resource via go-winres and injected via ldflags; if go-winres fails, the existing version resource is kept and the build continues. `go mod download` is always executed.
- **MSIX packaging**: Requires the WinApp CLI (`winget install Microsoft.WinAppCLI`); if the `devcert.pfx` certificate is missing, one is generated from the manifest and installed automatically. Every Windows GUI architecture that was built (`-Arch x64,x86,arm64`) is passed as payload, yielding a single mixed-architecture package at `build/bin/gui/Windows/<name>_<version>_<arch...>.msixbundle` (a plain `.msix` of the same name when only one architecture is built).
- **Linux build (WSL)**: `-Build linux` delegates to `build.sh` via WSL (GTK4 by default, `-Gtk3` switches to GTK3, `-Arch` forwarded as `--arch`); if WSL is not found, a warning is printed and the Linux build is skipped.

### Usage examples

```powershell
# Windows GUI, build everything (frontend + backend)
.\build_windows.ps1 -Build windows,gui,all

# Windows GUI, frontend only
.\build_windows.ps1 -Build windows,gui,frontend

# Linux GUI via WSL, build everything
.\build_windows.ps1 -Build linux,gui,all

# CLI only (headless, cross-platform)
.\build_windows.ps1 -Build windows,cli,all

# Both Windows + Linux GUI
.\build_windows.ps1 -Build all,gui,all

# Everything: all platforms, GUI + CLI
.\build_windows.ps1 -Build all,all,all

# arm64 build + MSIX packaging
.\build_windows.ps1 -Build windows,gui,all -Arch arm64 -BuildMsix

# Silent mode (for CI/CD, no interaction)
.\build_windows.ps1 -Silent

# Legacy format still works
.\build_windows.ps1 -Build frontend -Lang cn
.\build_windows.ps1 -Build all -BuildMsix -SkipSign

# No parameters = interactive mode
.\build_windows.ps1
```

---

## Linux Build

The Linux build uses the unified `build.sh` (interactive menu: GUI / CLI / both) and runs on a Linux host (or WSL2 on Windows). Windows users only need to run `build_windows.ps1` and do not have to worry about GTK dependencies.

### Dependencies (Ubuntu / Debian)

```bash
# GTK4 + WebKitGTK 6.0 (default, GUI only; the CLI build needs no GTK)
sudo apt-get update
sudo apt-get install -y libgtk-4-dev libwebkitgtk-6.0-dev

# Or use GTK3 + webkit2gtk-4.1
# sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

### Build Commands

```bash
# Clone the repository
git clone https://github.com/SnishaperTeam/SniShaper.git
cd SniShaper

# Interactive menu (1 GUI / 2 CLI / 3 GUI+CLI + architecture selection)
./build.sh

# GUI (uses existing frontend/dist)
./build.sh --gui

# Build frontend first, then backend
./build.sh --with-frontend

# Use GTK3 + webkit2gtk-4.1
./build.sh --gtk3

# CLI only (headless, cross-platform)
./build.sh --cli

# GUI + CLI
./build.sh --all

# Specify architecture (works for both GUI and CLI)
./build.sh --gui --arch arm64

# x86: Windows CLI only (Linux/Darwin skipped)
./build.sh --cli --arch x86
```

Build output is written to `build/bin/gui/Linux/<arch>/SniShaper` (including `rules/` and `config/` seed files). TUN / system proxy require root; run with `sudo ./build/bin/gui/Linux/x64/SniShaper`. CLI binaries land in `build/bin/cli/<Platform>/<Arch>/`.

---

## Version & Release Channel

The version number and release channel (`release` / `beta` / `alpha` / `rc`) are **unified** in the root `Package.appxmanifest`:

```xml
<rel:Version>1.29.0</rel:Version>
<rel:ReleaseChannel>beta.1</rel:ReleaseChannel>
```

Both the Windows and Linux builds read from this file and inject the values via ldflags (`snishaper/app.buildVersion`, `snishaper/app.buildChannel`). There is no separate version JSON file in the repository.

---

## Development Environment

- `Go 1.27+`
- `Node.js 24+` / `npm 11+`
- Windows: MSVC toolchain (Wails v3), WinApp CLI (MSIX packaging)
- Linux: GTK4 / WebKitGTK or GTK3 dev packages (see above)
- TUN mode depends on the gvisor network stack (enabled on Windows via the `with_gvisor` build tag)

Build outputs:

- Frontend assets at `frontend/dist`
- Windows GUI at `build/bin/gui/Windows/<arch>/snishaper.exe` (x64 by default)
- Linux GUI at `build/bin/gui/Linux/<arch>/SniShaper`
- CLI at `build/bin/cli/{Windows,Linux,Darwin}/<arch>/snishaper[.exe]`

---

## Continuous Integration

Dual-platform CI pipelines:

- **`build.yml`**: Triggered on every push / PR. Builds Windows on `windows-2025` and Linux on `ubuntu-24.04`, then runs compilation and a binary smoke test.
- **`_release_pipeline.yml`**: Release pipeline. The Windows runner produces the MSIX and `snishaper-windows-amd64.7z` portable archive, the Ubuntu runner produces `snishaper-linux-amd64.tar.gz`, and finally the Windows runner merges both platform artifacts and creates the GitHub Release. Release notes are generated first by a local Ollama instance on the runner (default `qwen3.5:2b`); when Ollama is unavailable, it falls back to a categorized commit list.

---

## Cross-Platform Notes

Windows and Linux are built from the same repository, with platform-specific implementations isolated via Go build tags (e.g. `//go:build linux` / `windows`). There is no separate Linux repository to visit.

The CLI (headless) build is maintained in the `cli/` subdirectory of this repository, sharing the same core code and versioning mechanism (`Package.appxmanifest`) with the GUI. It is built by `build.sh --cli` / `build_windows.ps1 -Cli`; both the CI and release pipelines produce GUI and CLI artifacts.
