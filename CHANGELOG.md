# Changelog

## v0.5.4 — 2026-11-03

### Security
- **Descriptor-relative traversal:** On Linux and macOS, snapshots are now read using `openat` with `O_NOFOLLOW` on every path component. This closes the time-of-check-to-time-of-use (TOCTOU) race where a parent directory could be swapped for a symlink between validation and read.
- **Parent-directory symlink defense:** Directory components are verified to be real directories at the moment of access, not just at validation time.
- **Dangling symlink rejection:** Files that are dangling symlinks are explicitly rejected rather than being treated as new files.
- **Strengthened race tests:** Added integration tests that repeatedly swap a directory with a symlink, verify no outside content is captured, and confirm rejected reads don't create provenance records.

### Added
- **macOS CI:** GitHub Actions now runs the full test suite and `go vet` on both Ubuntu and macOS. Release binaries are built for linux/darwin on both amd64 and arm64.

## v0.5.3 — 2026-11-03


### Security
- **Airtight symlink path traversal fix:** Resolves the deepest existing parent directory and validates it against the canonical repository root. Rejects targets that resolve outside the repo via file or directory symlinks, even for non-existent files.
- **Path traversal regression tests:** Added integration tests for file symlinks, directory symlinks, nested non-existent files, valid internal symlinks, and ordinary files.

## v0.5.2 — 2026-11-03


### Security
- **Symlink path traversal fix:** Validates that files resolved via symlinks cannot escape the repository boundary during snapshotting.
- **Action version bump:** Updated the composite GitHub Action and workflow examples to `v0.5.1`.

## v0.5.1 — 2026-11-03


### Security & Fixes
- **Path traversal hardening:** Validated that file paths resolved during IDE hook snapshots remain strictly inside the repository.
- **Snapshot collision resistance:** Replaced the slash-to-underscore snapshot naming scheme with SHA-256 hashes to prevent file overwrites.
- **Cursor limitation:** Documented that Cursor uses coarse (file-level) attribution because it lacks a pre-edit hook.
- **CI test fixes:** Generated ephemeral SSH keys in the integration tests to prevent CI failures on runners without `~/.ssh`.

### Added
- Regression tests for path traversal and snapshot collisions.

## v0.5.0 — 2026-11-03


### Security & Privacy
- **Opt-in prompt capture:** `capture_prompt` now defaults to `false`. Prompt text is only stored if explicitly enabled.
- **Expanded secret redaction:** Before writing prompts to disk, Auteur now redacts AWS keys, JWTs, private keys, connection strings, Slack tokens, and generic `password=`/`token=` patterns.
- **Authoritative trust root:** `auteur check --ci-base main` reads the trust root from the base branch, fails if it's missing, and rejects any PR that modifies it.
- **Line-level CI validation:** CI compares the exact changed lines in the PR diff against provenance records. If a line was changed but has no record, the build fails.
- **PreToolUse snapshotting:** IDE hooks capture file state before and after the AI edit. Human uncommitted changes are never attributed to the AI.
- **Safe IDE config merging:** `auteur init` merges hooks into existing `.claude/settings.json` and `.cursor/hooks.json` without destroying user config.
- **Fail-closed verification:** `check`, `verify`, and `sign` fail if the trust root is missing, empty, or doesn't contain the signer's key.
- **SSH-signed Git commits:** `auteur sign` creates an empty Git commit signed with your SSH key (`git commit -S`).
- **Signature payload fix:** Signatures no longer include `PrevHash`, allowing multi-reviewer approvals and out-of-order signing.

### Added
- `auteur trust list` and `auteur trust remove`.
- `auteur hook-agent-pre` for snapshotting.
- Regression tests for T1-T6 attacks, trust-root enforcement, line-level CI validation, PreToolUse snapshotting, prompt redaction, and CI trust root authority.

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
