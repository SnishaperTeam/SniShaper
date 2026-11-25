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
