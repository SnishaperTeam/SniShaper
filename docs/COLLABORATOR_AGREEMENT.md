# SniShaper Collaborator Agreement

- **Version:** 1.0
- **Effective Date:** 2026-10-08
- **Last Modified:** 2026-10-08

This document defines the terms and conditions under which a contributor may be invited to, and serve as, a collaborator of the SniShaper repository ([SnishaperTeam/SniShaper](https://github.com/SnishaperTeam/SniShaper)). Accepting an invitation constitutes agreement to every clause below.

## 1. Change Workflow

- **1.1** Direct pushes to the `main` branch are prohibited. All changes — including a collaborator's own — must go through pull requests.
- **1.2** A collaborator may merge their own pull request only after CI passes; non-trivial changes should allow a reasonable maintainer review window before merging.

## 2. Code Conventions

- **2.1** Follow the repository conventions defined in `AGENTS.md`.
- **2.2** Commit messages must be written in English using the conventional-commit style (e.g. `feat: ...`, `fix: ...`, `refactor: ...`).
- **2.3** The `zh` / `en` / `ru` i18n key trees must stay structurally consistent; new UI copy is added to all three locales in the same change.
- **2.4** Do not shell out through `cmd /c` (or an equivalent shell) with user-controlled input. Prefer direct system APIs (e.g. `ShellExecuteW`, `SHOpenFolderAndSelectItems`).

## 3. External Link Policy

- **3.1** External URLs and files must be opened through the system handler (`common.OpenTarget`), so they open in the system default browser or the associated default application.
- **3.2** Opening external content inside the application WebView is prohibited.

## 4. Security-Sensitive Areas

- **4.1** Changes to security-sensitive areas — certificate management, TUN, system proxy, and core RPC token handling — require explicit maintainer review and approval before merge.
- **4.2** Suspected vulnerabilities must be reported as described in [SECURITY.md](SECURITY.md), not disclosed publicly.

## 5. Invitation and Acceptance

- **5.1** An invitation is extended by a maintainer, usually as a comment on a merged pull request.
- **5.2** To accept, reply **"Yes"** to the invitation comment. To decline, reply **"No"**.
- **5.3** An invitation is valid for **3 days (72 hours)** from the time the invitation comment is posted. After expiry the invitation is void and must be re-issued to take effect.

## 6. Modification of Terms

- **6.1** The maintainers may modify this agreement at any time; material changes will be announced to current collaborators.
- **6.2** Each modification updates the **Last Modified** date above. Continued collaboration after a modification takes effect constitutes acceptance of the modified terms.

## 7. Termination

- **7.1** Collaborator access may be revoked by the maintainers at any time for violation of this agreement, the [Code of Conduct](CODE_OF_CONDUCT.md), or behavior harmful to the project.
