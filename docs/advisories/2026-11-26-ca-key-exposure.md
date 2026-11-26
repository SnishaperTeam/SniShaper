# Security Advisory: Exposed development CA key in git history

- **Date**: 2026-11-26
- **Affects**: repository source archives of v1.27 / v1.28 (June–August 2026)
- **Fixed in**: v1.30.3.3 (history purged; hardened release pipeline)
- **Severity**: low for end users — the key was never used by the application
- **CVE**: none assigned

## 中文版

### 摘要

2026-06-13，一次"备份本地改动"的提交误将一组开发用证书（`CN=SniShaper
CA`，RSA-2048）连同**私钥**提交进了仓库的 `build/cert/` 目录。该文件已于
2026-08-15 删除，但在历史重写之前，它始终可从 git 历史恢复，并且存在于
**v1.27 与 v1.28 两个 tag 的自动生成源码归档**中，可被公开下载。

### 影响

- 对全部相关提交的静态分析确认：**没有任何代码引用过 `build/cert/`**。
  该私钥从未被程序加载、从未进入任何发行二进制、从未用于签署任何发布产物。
- 应用的 MITM 根证书由每个用户首次运行时在本机生成，存放于用户配置目录，
  与泄漏的密钥无关。
- MSIX 签名证书（devcert.pfx）与该密钥无关。
- 因此**没有已知的可利用路径**。但该私钥曾公开可下载，任何名为
  "SniShaper CA" 的旧证书链都应视为不可信。

### 已采取的措施

1. **历史重写**：已用 git filter-repo 从全部历史中清除该私钥，所有 tag
   已重新推送。旧提交哈希在本仓库中不再存在。
2. **密钥作废**：该 CA 视为已烧毁（burned），任何代码不得信任、不得复用。
3. **供应链加固**：CI 全部 action 固定到 commit SHA；linuxdeploy 下载固定
   版本并校验 sha256；发布流水线对签名证书密码 fail-fast；每个发布资产
   附带 `.sha256` 校验文件。
4. **热更新 v1.30.3.3** 发布，包含上述加固。

### 用户需要做什么

- **无需恐慌性操作**：应用从不要求你导入来自仓库下载的 "SniShaper CA"
  证书。
- 如果你在 v1.27/v1.28 时期手工把这样一个根证书导入过系统或浏览器信任库，
  请删除它。
- 建议升级到 v1.30.3.3 或更新版本。

## English version

### Summary

On 2026-06-13 a "backup local changes" commit accidentally introduced a
development certificate authority (`CN=SniShaper CA`, RSA-2048) **with its
private key** into the repository under `build/cert/`. The file was deleted
on 2026-08-15, but until the history rewrite it remained recoverable from
git history and was present in the **auto-generated source archives of tags
v1.27 and v1.28**, publicly downloadable.

### Impact

- Static analysis of every commit between introduction and deletion
  confirms **no code ever referenced `build/cert/`**. The key was never
  loaded by the application, never shipped inside any binary, and never
  used to sign any release artifact.
- The app's MITM root CA is generated per-user at first run under the
  user config directory — a different key, unaffected.
- The MSIX signing certificate (`devcert.pfx`) is unrelated.
- **No known exploitable path.** However, since the key was publicly
  retrievable, any certificate chain claiming to be "SniShaper CA" from
  that era must be treated as untrusted.

### What we did

1. **History rewrite**: purged the key from all history with git
   filter-repo and re-pushed every tag. Old commit hashes no longer
   resolve in this repository.
2. **Key burned**: the exposed CA is retired; no code may trust or reuse
   it.
3. **Supply-chain hardening**: all CI actions pinned to commit SHAs;
   linuxdeploy pinned to a fixed release with sha256 verification;
   fail-fast on a missing signing-certificate password; per-asset
   `.sha256` checksums published with every release.
4. Released hotfix **v1.30.3.3** shipping the hardened pipeline.

### What users should do

- **No urgent action required**: the application never asks you to import
  a "SniShaper CA" certificate obtained from repository downloads.
- If you manually imported such a root certificate while experimenting
  with v1.27/v1.28-era sources, remove it from your OS/browser trust
  store.
- Update to v1.30.3.3 or later.

---

Reported and resolved via the private channels listed in
[SECURITY.md](../SECURITY.md).
