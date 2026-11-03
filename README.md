# Auteur

**Provenance ledger for AI-authored code.**

![Auteur demo](docs/demo.gif)

git blame tells you *who* wrote a line. Auteur tells you *whether it was
a human or a model* — and which model, session, and prompt hash produced it.

    $ auteur blame auth/login.py:42
    auth/login.py:42
      author:  claude-code (claude-opus-5.5)
      session: sess_abc123
      commit:  62e291c  (2026-10-31)
      range:   40-55
      origin:  agent   [confidence: high]

As AI writes more of our code, "who touched this line" stops being enough.
When a bug ships at 3am, you need to know whether the fix came from a human
or a model, which model, and whether anyone reviewed it. Auteur records
that alongside your git history.

*The name: an "auteur" is the author whose personal creative vision shapes
a work. Here it's the model, the session, and the person who reviewed the
result.*

## How it works

Auteur keeps a ledger of which line ranges in which commits were authored
by an agent, and with what confidence:

- high   — recorded by `auteur wrap` (the agent ran under Auteur)
- medium — inferred from Co-authored-by trailers in the commit message
- low    — reserved for future heuristics
- unknown — never assume human; absence of a record means unknown

The ledger is a plain text file (.auteur/provenance.jsonl) committed with
your repo. No server, no database, no telemetry.

## Install

    go install github.com/stythelad/auteur/cmd/auteur@latest

Or build from source:

    git clone git@github.com:stythelad/auteur.git
    cd auteur
    make install

## Quickstart

In any git repository:

    auteur init

That creates .auteur/config.yaml, .auteur/policy.yaml, an empty
.auteur/provenance.jsonl, and installs a post-commit hook.

### Record agent work

    auteur wrap --agent claude-code --model claude-opus-5.5 -- claude "fix the login bug"
    git add .
    git commit -m "fix login bug"
    auteur blame auth/login.py:42

The post-commit hook resolves the pending record to your commit's SHA.

### Bootstrap existing history

Already have agent commits? Scan for Co-authored-by trailers:

    auteur scan-history --dry-run
    auteur scan-history

### Gate CI on policy

    auteur check --pr-base main

Exits non-zero on violations.

## Policy

.auteur/policy.yaml supports four rule types:

    version: 1
    rules:
      - name: protected-paths-need-signoff
        paths: ["auth/**", "payments/**", "**/crypto/**"]
        require_human_signoff: true

      - name: no-agent-code-in-migrations
        paths: ["migrations/**", "**/migrations/**"]
        forbid_agent_authored: true

      - name: ci-must-have-passed
        paths: ["**"]
        require_ci_passed: true

      - name: provenance-required-in-sensitive-areas
        paths: ["auth/**", "payments/**"]
        require_provenance: true

Rule fields: require_human_signoff, forbid_agent_authored, require_ci_passed,
require_provenance. At least one must be set per rule.

Path matching supports *, ?, and ** (multi-segment).

`auteur check --since <ref>` only checks commits not reachable from `<ref>`,
which is what you want in CI.

## Approving agent code

By default, `auteur check` fails closed. If an agent touches a protected path
(e.g. `auth/`), the build fails until a human signs off on that specific commit.

To approve a commit, use the CLI:

    auteur sign <commit-sha> --user "alice"

This updates the `human_signoff` array for that specific record in the ledger.
You then commit the updated `.auteur/provenance.jsonl` file to your branch.
The `post-commit` hook does not do this automatically.

### Tamper-evident ledger

To prevent developers from bypassing the policy by just hand-editing the JSONL
file to add their own name, the ledger is **tamper-evident**. Every record
includes a SHA-256 hash of the previous record. If the file is modified outside
of `auteur`, the hash chain breaks, and `auteur check` will fail instantly with
a `ledger tampering detected` error.

## CI integration

GitHub Actions:

    name: auteur
    on: [pull_request]
    jobs:
      check:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
            with:
              fetch-depth: 0
          - uses: actions/setup-go@v5
            with:
              go-version: '1.22'
          - run: go install github.com/stythelad/auteur/cmd/auteur@latest
          - run: auteur check --since origin/${{ github.base_ref }}

fetch-depth: 0 is required so Auteur can see the base branch history.

## Command reference

| Command | Purpose |
|---|---|
| `auteur init` | Create ledger, config, policy, install post-commit hook |
| `auteur wrap -- <cmd>` | Run a command and record its line changes as pending |
| `auteur blame <file>:<line>` | Show provenance for a single line |
| `auteur check` | Evaluate policy against the ledger |
| `auteur scan-history` | Bootstrap records from commit trailers |
| `auteur pending list` | Show unresolved pending records |
| `auteur pending show <uuid>` | Inspect a single pending record |
| `auteur pending gc` | Prune stale pending records |
| `auteur resolve-pending` | Resolve pending records to HEAD |
| `auteur rewrite <old> <new>` | Copy attribution to rewritten SHAs (manual) |

## Design notes

**Why a sidecar ledger, not git notes?**
Git notes don't survive rebases cleanly and are awkward to query. A JSONL
file committed with the repo is diffable, mergeable, and travels with
every clone.

**Why JSONL, not a single JSON array?**
Append-only writes are O(1) and crash-safe: a partial write corrupts at
most the last line.

**Why git stash create for snapshots?**
stash create is O(tracked files) and doesn't touch disk beyond one commit
object. File copying is O(repo size).

**Why confidence tiers?**
A tool that claims perfect attribution is lying. medium from trailers is
real signal, but it's not the same as a wrap-recorded high.

## Roadmap

- [x] Ledger store, init
- [x] blame with reverse-walk lookup
- [x] wrap with diff extraction
- [x] Post-commit hook to resolve pending records
- [x] Policy engine (check)
- [x] Trailer-based history scan
- [x] Rebase survival for pending records (v0.2)
- [ ] Editor integration (VS Code, Neovim)
- [ ] Agent-native SDK
- [ ] blame --format=oneline for scripting

## Status

v0.2 — usable, opinionated, early. Breaking changes possible before v1.

## Prior art

Auteur is not the only line-level AI attribution tool. Worth knowing about:

- git-ai — git notes based, agent hooks, rebase survival
- GitIntelAI — Rust CLI, zero-setup trailer detection, cost tracking
- Agent Blame — git notes, browser extensions
- agentdiff — ed25519-signed records

Auteur differentiates on: explicit confidence tiers, a policy engine for
CI gating, rebase/amend survival, and a both-ways approach (wrap for high
confidence, trailers for medium).

## Known footguns

**Stale binary + silent hooks.** The git hooks installed by `auteur init`
call `auteur` from `$PATH`. After upgrading Auteur, rebuild and re-install
the binary; otherwise hooks silently no-op (`|| true`). If attribution
isn't appearing after a commit, run `auteur resolve-pending --verbose`
manually and check `which auteur` points at the new build.

**Ledger commits and rebases.** `.auteur/provenance.jsonl` is a tracked
file. If you commit it on a feature branch and then rebase, git may
complain about local changes. Either `git rebase --autostash`, or add
`.auteur/` to `.gitignore` during development and commit it periodically.

## License

MIT. See LICENSE.

## Contributing

See CONTRIBUTING.md. Issues and PRs welcome.
