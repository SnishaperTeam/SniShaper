# Security Policy

## Supported Versions

We provide security fixes for the actively maintained release lines:

| Version | Supported |
| ------- | --------- |
| 1.31.x pre-releases (alpha / beta / rc) | ✅ (best effort) |
| 1.30.x (latest stable line) | ✅ |
| < 1.30 | ❌ — upgrade to the latest release first |

Pre-release builds ship through the alpha / beta / rc channels and may contain
known issues. Security problems found in them are still in scope — please
report them.

## Reporting a Vulnerability

**Do not open a public issue, pull request, or public Matrix message for
security problems.** Report privately via either channel:

1. **GitHub private vulnerability reporting** (preferred):
   open the repository's **Security** tab and click **Report a vulnerability**
   (<https://github.com/SnishaperTeam/SniShaper/security/advisories/new>)
2. **Email**: [SniShaper@proton.me](mailto:SniShaper@proton.me)

Include as much of the following as you can:

- Affected version(s) — the app version from the About page or the release tag
- Component or file(s) involved
- Step-by-step reproduction or a proof of concept
- Assessed impact (crash, RCE, privilege escalation, data exposure, ...)

### What to expect

- We aim to acknowledge reports within **72 hours**.
- You will receive status updates while the issue is investigated and fixed.
- Accepted reports are fixed in a patch release and, where applicable,
  published as a GitHub Security Advisory with credit to the reporter
  (anonymous credit on request).
- Please allow time for a coordinated fix and release before any public
  disclosure.

## Scope

- **In scope**: everything in this repository — Go backend (`/app`, `/core`,
  `/pkg`), React frontend, build scripts, CI configuration, packaging.
- **Engine vulnerabilities** (ECH injection, TLS fragmentation, QUIC
  connection rebuilding, TUN stack, ...) live in
  [snishaper-core](https://github.com/SnishaperTeam/snishaper-core). Report
  them there, or by email — we will route them.
- **Dependency CVEs** in third-party modules are tracked via Dependabot and
  patched in routine dependency updates. Unpatched upstream issues belong to
  the upstream project.

### By design, not a vulnerability

SniShaper deliberately rewrites, fragments and rebuilds TLS/QUIC traffic to
adapt to network middleboxes. This behavior is the product's intended
function; reports that it "bypasses DPI" or "evades censorship" will be
declined.

## Safe Harbor

We consider good-faith security research to be in the public interest. We
will not pursue legal action against researchers who act with respect for
user privacy and avoid service degradation, data destruction, and data
exposure.

## Incident Record: exposed development CA key (resolved 2026-11-26)

On 2026-06-13 a local backup commit accidentally introduced a development
certificate authority (`CN=SniShaper CA`, RSA-2048) **including its private
key** into `build/cert/`. The file was deleted on 2026-08-15, but until the
history rewrite below it remained recoverable from git history and was
present in the auto-generated **source archives of tags v1.27 and v1.28**.

Impact assessment:

- Static analysis of every commit between introduction and deletion shows
  **no code ever referenced `build/cert/`**; the key was never loaded by
  the application, never shipped inside any binary, and never used to sign
  any release artifact.
- The app's MITM root CA is generated per-user at first run and stored
  under the per-user config directory — it is a different key and is
  unaffected.
- The MSIX signing certificate (`devcert.pfx`) is unrelated to the exposed
  key.

Resolution (2026-11-26):

- The entire repository history was rewritten with `git filter-repo` to
  purge the key; all tags were re-pushed. Old commit hashes no longer
  resolve in this repository.
- The exposed CA is treated as **burned**: no SniShaper code trusts it,
  and it must never be reused.
- Release v1.30.3.3 ships the hardened CI pipeline (SHA-pinned actions,
  per-asset SHA-256 checksums).

**User action:** the application never asks you to import a certificate
named "SniShaper CA" from a repository download. If you manually imported
such a root certificate while experimenting with v1.27/v1.28-era sources,
remove it from your OS/browser trust store and update to the latest
release. No other user action is required.
