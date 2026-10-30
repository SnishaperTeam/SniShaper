# SniShaper TODO

This file tracks planned work only. Existing features are intentionally omitted unless further work is explicitly planned here.

## Native Desktop GUI

- [ ] Replace the WebView-based desktop interface with a native GUI.
- [ ] Preserve feature parity with the existing interface and reuse the existing Go core instead of duplicating proxy logic.
- [ ] Preserve existing configuration files and provide a safe migration path if the GUI requires configuration changes.
- [ ] Prioritize low memory usage, fast startup, accessibility, high-DPI displays, system theme support, and native window/tray behavior.
- [ ] Evaluate GUI frameworks against the project's supported platforms and maintenance costs before committing to one.

## macOS GUI

- [x] Evaluate and implement a native macOS GUI.
- [x] Reuse the shared core and existing configuration formats wherever practical.
- [x] Integrate with macOS system proxy settings and clearly explain permissions or platform limitations.
- [x] Build and test on Apple Silicon and, where feasible, Intel Macs.
- [x] Document installation, upgrades, removal, and known limitations.

## Live Connection Viewer

- [x] Display active connections and their current states.
- [x] Show available connection details, including domain, destination IP and port, matched rule, selected proxy mode, and traffic counters.
- [x] Make it clear when a field is unavailable or inferred rather than presenting it as confirmed information.
- [x] Support filtering and searching connections, plus copying useful details for troubleshooting.
- [x] Ensure connection inspection has bounded memory use and does not expose sensitive data unnecessarily.

## Rule Match Explanation

- [x] Add a tool that accepts a domain or destination and explains how SniShaper would handle it.
- [x] Show the matched rule, the resulting proxy mode, and relevant DNS and routing decisions.
- [x] Explain rule priority and why a more general rule did not win when that information is available.
- [x] Distinguish a simulated result from the decision made for a real connection.
- [x] Keep the explanation consistent with the actual rule engine to avoid maintaining separate, conflicting matching logic.

## Diagnostic Export

- [x] Provide a one-click way to export a diagnostic package for bug reports.
- [x] Include relevant application version and build information, sanitized logs, network status, and configuration details needed to reproduce problems.
- [x] Redact passwords, tokens, private keys, credentials, and other sensitive values before packaging files.
- [x] Preview the files and sensitive fields that will be included before export.
- [x] Document the package format and verify that export failures do not affect the running proxy.

## Network Failure Recovery

- [x] Detect and report network settings that may have been left behind after an unexpected exit or crash.
- [x] Provide a safe recovery process for SniShaper-managed system proxy settings, routes, and DNS changes.
- [x] Record enough state to distinguish settings created by SniShaper from settings owned by the user or other applications.
- [x] Restore only changes that SniShaper can confidently identify as its own; never blindly overwrite user settings.
- [x] Test recovery after forced termination, failed startup, interrupted shutdown, and system restart.
- [x] Show what was detected and changed, and provide actionable error messages if administrator permissions are required.

## Network Regression Test Suite

- [x] Add automated regression tests for TUN startup, shutdown, restart, and repeated enable/disable cycles.
- [x] Test DNS resolution, IPv4 and IPv6 behavior, route installation and cleanup, and system proxy restoration.
- [x] Test concurrent connections, connection churn, long-running sessions, and resource usage over time.
- [x] Cover startup failures, interrupted shutdowns, permission failures, and recovery from partial setup.
- [x] Run applicable tests in CI and document tests that require particular operating systems or elevated permissions.

## Linux Package Formats

- [x] Evaluate Linux distribution formats in addition to the existing archive distribution.
- [x] Assess AppImage, Debian packages (`.deb`), and RPM packages based on maintenance cost and distribution coverage.
- [x] Ensure packages install the correct binaries, desktop entries, icons, and required metadata.
- [x] Define upgrade and uninstall behavior, including cleanup of application-owned files without deleting user data.
- [x] Document supported distributions and clearly state any runtime or permission requirements.

## Multiple Configuration Profiles

- [x] Support multiple named configuration profiles for different networks or usage scenarios.
- [x] Allow users to create, rename, duplicate, switch, import, and export profiles.
- [x] Keep each profile's rules, DNS settings, and proxy mode isolated from other profiles.
- [x] Validate an imported profile before applying it and explain invalid or conflicting settings.
- [x] Define safe behavior when switching profiles while connections are active, including whether a restart is required.
- [x] Preserve existing configurations when introducing profile support.

## Per-Application Routing

- [ ] Investigate per-application proxying so users can select applications to proxy or bypass.
- [ ] Define platform-specific support and limitations before exposing the feature in the UI.
- [ ] Specify how application identity is detected and how rules interact with domain-based and IP-based routing.
- [ ] Provide clear feedback when an application cannot be identified or per-application routing is unsupported.
- [ ] Add tests for conflicts between application, domain, IP, and default routing rules.

## Proxy Performance Benchmarks

- [ ] Create repeatable benchmarks for supported proxy modes and relevant network paths.
- [ ] Measure throughput, latency, CPU usage, and memory usage under defined test conditions.
- [ ] Include representative workloads such as short connections, sustained transfers, and many concurrent connections.
- [ ] Record test environment and configuration so results can be compared across releases.
- [ ] Use benchmark results to validate performance changes rather than treating a single run as conclusive.

## Configuration Migration and Compatibility

- [ ] Add explicit configuration versioning and migration steps where needed.
- [ ] Check old configurations for removed fields, invalid values, conflicting rules, and unsupported options during upgrades.
- [ ] Provide a clear migration report describing changes, warnings, and errors.
- [ ] Preserve a backup of the original configuration before applying a migration.
- [ ] Avoid silently changing behavior when a configuration cannot be migrated safely.
- [ ] Add compatibility tests using representative configurations from previous releases.
