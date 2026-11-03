# Changelog

## v0.4.0 — 2026-11-03

### Security (Massive overhaul based on external audit)
- **Committed trust root:** Signers are now managed in a committed `.auteur/allowed_signers` file via `auteur trust add`.
- **Fail-closed verification:** If a signoff exists but no trusted signers are configured, `check` and `verify` fail.
- **CI policy enforcement (`--ci-base`):** In CI, `auteur check --ci-base main` reads both the policy and trust root from the base branch. A PR can no longer modify its own policy or add its own signer.
- **File-level provenance check:** CI now checks all *changed files* in protected paths. If an attacker bypasses the local hooks (e.g. `git commit --no-verify`), the file will have no record and CI will fail.
- **Signature payload fix:** The signature hash no longer includes `PrevHash`, fixing out-of-order signing and allowing multi-reviewer approvals.
- **Untrusted key block:** `auteur sign` refuses to sign if your key isn't in the trust root.
- **Config error handling:** YAML typos in the policy or trust root now fail the build instead of silently disabling security.
- **Corrupt trust file protection:** `auteur trust add` fails on a malformed YAML file instead of wiping the list.
- **SSH-signed Git commits:** `auteur sign` now creates an empty Git commit signed with your SSH key (`git commit -S`), providing a secondary layer of verification.

## v0.3.0 — 2026-11-03


### Added
- **Cryptographic identity verification:** `auteur sign` now uses local SSH keys to sign records. You cannot forge a signoff.
- **Tamper-evident ledger:** The ledger uses a SHA-256 hash chain. Manual edits break the chain and fail CI.
- **Native Claude Code & Cursor hooks:** `auteur init` installs zero-config hooks. Attribution happens automatically.
- **Line-level precision for hooks:** Agent hooks record exactly which lines were changed, not just the whole file.
- **Deep provenance metadata:** Hooks capture prompt snippets and session IDs. `auteur blame` displays this.
- **Cherry-pick survival:** Attribution is preserved when commits are cherry-picked.
- **Composite GitHub Action:** `.github/actions/auteur` provides a drop-in CI gate.
- **Worktree support:** The ledger moved to `.git/auteur/` to support concurrent agents.

### Changed
- `auteur check` now cryptographically verifies all signatures before evaluating policy.
- Removed the `--user` flag from `auteur sign` (identity is derived from your SSH key).

## v0.2.0 — 2026-11-01


### Added
- `post-rewrite` hook: attribution survives `git rebase` and `git commit --amend`
- `auteur rewrite` command for manual history-rewrite recovery
- Strict policy schema validation (unknown YAML fields rejected)
- `require_provenance` policy rule: unknown-origin lines fail in protected paths
- `--no-strict` and `--allow-missing-ledger` flags for `auteur check`
- Integration test suite covering wrap, blame, check, scan-history, amend, rebase

### Changed
- `auteur check` fails on unknown-origin lines by default (was: only with `--strict`)
- `auteur check` fails when `.auteur/provenance.jsonl` is missing (was: silently passed)
- `go.mod` pinned to Go 1.22 for broader installability

### Fixed
- Mixed-origin lines now trigger policy rules (were silently skipped)
- Attribution no longer lost after `git rebase` or `git commit --amend`

## v0.1.0 — 2026-10-31

Initial release. Core loop: init, wrap, blame, check, scan-history.
