# Sentinel

**Provenance ledger for AI-authored code.**

git blame tells you *who* wrote a line. Sentinel tells you *whether it
was a human or a model* — and which model, session, and prompt hash
produced it.

    $ sentinel blame auth/login.py:42
    auth/login.py:42
      author:  claude-code (claude-opus-5.5)
      session: sess_abc123
      commit:  62e291c  (2026-10-31)
      range:   40-55
      origin:  agent   [confidence: high]

As AI writes more of our code, "who touched this line" stops being
enough. When a bug ships at 3am, you need to know whether the fix came
from a human or a model, which model, and whether anyone reviewed it.
Sentinel records that alongside your git history.

## How it works

Sentinel keeps a ledger of which line ranges in which commits were
authored by an agent, and with what confidence:

- high   — recorded by sentinel wrap (the agent ran under Sentinel)
- medium — inferred from Co-authored-by trailers in the commit message
- low    — reserved for future heuristics
- unknown — never assume human; absence of a record means unknown

The ledger is a plain text file (.sentinel/provenance.jsonl) committed
with your repo. No server, no database, no telemetry.

## Install

    go install github.com/pokhrelmishan/sentinel/cmd/sentinel@latest

Or build from source:

    git clone git@github.com:pokhrelmishan/sentinel.git
    cd sentinel
    make install

## Quickstart

In any git repository:

    sentinel init

That creates .sentinel/config.yaml, .sentinel/policy.yaml, an empty
.sentinel/provenance.jsonl, and installs a post-commit hook.

### Record agent work

    sentinel wrap --agent claude-code --model claude-opus-5.5 -- claude "fix the login bug"
    git add .
    git commit -m "fix login bug"
    sentinel blame auth/login.py:42

The post-commit hook resolves the pending record to your commit's SHA.

### Bootstrap existing history

Already have agent commits? Scan for Co-authored-by trailers:

    sentinel scan-history --dry-run
    sentinel scan-history

### Gate CI on policy

    sentinel check --pr-base main

Exits non-zero on violations.

## Policy

.sentinel/policy.yaml supports three rule types:

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

Path matching supports *, ?, and ** (multi-segment).

sentinel check --since <ref> only checks commits not reachable from
<ref>, which is what you want in CI.

## CI integration

GitHub Actions:

    name: sentinel
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
          - run: go install github.com/pokhrelmishan/sentinel/cmd/sentinel@latest
          - run: sentinel check --since origin/${{ github.base_ref }}

fetch-depth: 0 is required so Sentinel can see the base branch history.

## Command reference

| Command | Purpose |
|---|---|
| sentinel init | Create ledger, config, policy, install post-commit hook |
| sentinel wrap -- <cmd> | Run a command and record its line changes as pending |
| sentinel blame <file>:<line> | Show provenance for a single line |
| sentinel check | Evaluate policy against the ledger |
| sentinel scan-history | Bootstrap records from commit trailers |
| sentinel pending list | Show unresolved pending records |
| sentinel pending show <uuid> | Inspect a single pending record |
| sentinel pending gc | Prune stale pending records |
| sentinel resolve-pending | Resolve pending records to HEAD |

## Design notes

**Why a sidecar ledger, not git notes?**
Git notes don't survive rebases cleanly and are awkward to query. A
JSONL file committed with the repo is diffable, mergeable, and travels
with every clone.

**Why JSONL, not a single JSON array?**
Append-only writes are O(1) and crash-safe: a partial write corrupts at
most the last line.

**Why git stash create for snapshots?**
stash create is O(tracked files) and doesn't touch disk beyond one
commit object. File copying is O(repo size).

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
- [ ] Editor integration (VS Code, Neovim)
- [ ] Agent-native SDK
- [ ] blame --format=oneline for scripting

## Status

v0.1 — usable, opinionated, early. Breaking changes possible before v1.

## License

MIT. See LICENSE.

## Contributing

See CONTRIBUTING.md. Issues and PRs welcome.
