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

The ledger is a plain text file (.git/auteur/provenance.jsonl) committed with
your repo. No server, no database, no telemetry.

### Zero-config agent capture

You don't need to wrap your agents. `auteur init` installs native hooks for:

- **Claude Code** (`.claude/settings.json`)
- **Cursor** (`.cursor/hooks.json`)

When they edit a file, the hook diffs it in the background and records exactly
which lines changed (including the session ID and prompt hash).
For other agents (Copilot, Codex, etc.), use `auteur wrap`.


## Install

    go install github.com/stythelad/auteur/cmd/auteur@latest

Or build from source:

    git clone git@github.com:stythelad/auteur.git
    cd auteur
    make install

## Quickstart

In any git repository:

    auteur init

That creates .git/auteur/config.yaml, .git/auteur/policy.yaml, an empty
.git/auteur/provenance.jsonl, and installs a post-commit hook.

### Record agent work

**Automatic (Claude Code):**
If you use Claude Code, you don't need to wrap it. `auteur init` automatically installs a `.claude/settings.json` hook. When Claude edits a file, the hook quietly records it. When you commit, Auteur attributes those files to Claude at `high` confidence.

**Automatic (Cursor):**
Cursor is also supported out of the box. `auteur init` installs a `.cursor/hooks.json` file. Any file edited by Cursor's Agent or Composer is automatically attributed when you commit.

**Manual (Copilot, Codex, Any Other Agent):**
For tools that don't have local lifecycle hooks, wrap the invocation. This captures the exact line ranges changed during the session:

    auteur wrap --agent copilot -- copilot-cli "fix the login bug"
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

.git/auteur/policy.yaml supports four rule types:

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

When running in CI, use `--ci-base` to read both the policy and the trust root
from the base branch, and to check all changed files in protected paths:

    auteur check --ci-base origin/main

This ensures that:
1. The policy is read from `main`, not the PR (so a PR cannot empty the policy).
2. The trust root is read from `main`, not the PR (so a PR cannot add its own signer).
3. Every changed file in a protected path must have a provenance record. If a file
   was modified but has no record, the build fails (so AI code cannot be committed
   without attribution).

## Approving agent code

By default, `auteur check` fails closed. If an agent touches a protected path
(e.g. `auth/`), the build fails until a human signs off on that specific commit.

To approve a commit, use the CLI:

    auteur sign <commit-sha>

This cryptographically signs the commit's provenance record using your local
SSH key (default: `~/.ssh/id_ed25519`). The ledger stores your identity, your
public key, and the cryptographic signature. 

Auteur verifies that the signature matches the key used to create it. However, to
prevent someone from generating a throwaway SSH key and claiming to be a
maintainer, you **must** configure a list of trusted signers in your config.

Add the allowed identities and their public SSH keys to `.git/auteur/config.yaml`:

    trusted_signers:
      "Alice": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI..."
      "Bob": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI..."

When `auteur check` or `auteur verify` runs, it will reject any signature
where the public key does not match the trusted key for that identity in the
config. The signoff is saved directly to the local `.git/auteur/provenance.jsonl` file.

### Tamper-evident ledger

To prevent developers from bypassing the policy by just hand-editing the JSONL
file to add their own name, the ledger is **tamper-evident**. Every record
includes a SHA-256 hash of the previous record. If the file is modified outside
of `auteur`, the hash chain breaks, and `auteur check` will fail instantly with
a `ledger tampering detected` error.

## Privacy controls

Because agent hooks capture prompt text from your IDE, Auteur includes strict
privacy controls. By default, it automatically redacts API keys and tokens,
and truncates prompts to 200 characters.

You can configure this in `.git/auteur/config.yaml`:

    privacy:
      capture_prompt: true      # Set to false to only store a SHA-256 hash of the prompt
      prompt_max_length: 200    # Truncate long prompts
      redact_secrets: true      # Automatically mask API keys/tokens (sk-..., ghp_..., etc.)
      capture_session_id: true  # Store the IDE session ID

If you are in a highly regulated environment (e.g. SOC2, HIPAA), set
`capture_prompt: false`. Auteur will still track exactly which lines were
written by the AI, but it will never store the prompt text.

## CI integration

We provide a composite GitHub Action that handles installation and execution.

    name: Auteur
    on: [pull_request]
    jobs:
      check:
        runs-on: ubuntu-latest
        steps:
          - uses: actions/checkout@v4
            with:
              fetch-depth: 0
          - uses: stythelad/auteur@v0.3.0
            with:
              base-ref: ${{ github.base_ref }}
              auteur-version: 'v0.3.0' # Optional: defaults to the release tag

`fetch-depth: 0` is required so Auteur can see the base branch history.

## Cryptographic auditing

Because Auteur makes security claims, you can explicitly audit the ledger:

    auteur verify

This runs a deep audit and outputs the trust guarantees:

    === Auteur Ledger Audit ===
    Total records: 1

    [1] Integrity (SHA-256 Hash Chain)
      Status: PASSED
      The ledger has not been tampered with.

    [2] Authenticity (SSH Signatures)
      Status: PASSED
      All cryptographic signatures are valid.

    [3] Signoff Roster
      61d85ee: Alice

    === Trust Model ===
    - The hash chain proves this ledger has not been accidentally corrupted or hand-edited.
    - The SSH signatures prove which local keys authorised which exact code changes.
    - Note: This tool assumes the signing keys are not compromised. It does not replace CODEOWNERS or human review.

If someone manually edits the JSONL file to change a signer's identity or line
ranges, `auteur verify` will instantly fail with a `hash mismatch` and an
`invalid signature` error.

### Managing the trust root

You must explicitly authorize the SSH keys that are allowed to sign off on AI code.
This prevents an attacker from generating a throwaway key and claiming to be you.

    auteur trust add "Alice" ~/.ssh/id_ed25519.pub
    auteur trust add "Bob" ~/.ssh/id_ed25519.pub
    auteur trust list
    auteur trust remove "Bob"

This manages the `.auteur/allowed_signers` file. You must commit this file to
your repository. We recommend protecting it with a `CODEOWNERS` rule so that
only maintainers can approve changes to it.

## Command reference

| Command | Purpose |
|---|---|
| `auteur init` | Create ledger, config, policy, install post-commit hook |
| `auteur sign <sha>` | Cryptographically sign off on an agent commit |
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

**Why a ledger in `.git/auteur/`?**
Storing the ledger in the working tree caused merge conflicts and broke
`git worktree`. By moving it to `.git/auteur/`, it is shared safely across
all worktrees and never accidentally committed. The ledger is a JSONL file
that is append-only and crash-safe.

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

v0.3 — usable, opinionated, early. Breaking changes possible before v1.

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


## License

MIT. See LICENSE.

## Contributing

See CONTRIBUTING.md. Issues and PRs welcome.
