# Sentinel

Provenance ledger for AI-authored code.

`git blame` tells you **who** wrote a line. Sentinel tells you
**whether it was a human or a model**, and which model, session, and
prompt hash produced it.

## Status

Early. v1 in progress. The ledger format is stable enough to test
against; the CLI surface will grow.

## Install

    go install github.com/pokhrelmishan/sentinel/cmd/sentinel@latest

## Usage (so far)

    cd your-repo
    sentinel init

This creates:

    .sentinel/config.yaml
    .sentinel/provenance.jsonl

## Design

- Source of truth: append-only JSONL ledger committed with the repo.
- One record per commit, line-range based.
- Confidence tiers: `high` (recorded via wrapper), `medium` (git trailers),
  `low` (heuristics). Unknown lines are `unknown`, never assumed human.

## Roadmap

- [x] Ledger + `init`
- [ ] `sentinel blame <file>:<line>`
- [ ] `sentinel wrap -- <agent-cmd>`
- [ ] `sentinel check --policy policy.yaml`
- [ ] Heuristic attribution from commit trailers
- [ ] GitHub Action
