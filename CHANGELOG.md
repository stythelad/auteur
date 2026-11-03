# Changelog

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
