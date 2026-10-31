# Changelog

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
