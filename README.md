# Auteur

**Track code provenance in AI-assisted repositories.**

Auteur records which changed line ranges are attributed to an AI agent, the
evidence used for that attribution, and whether a trusted human signer approved
the associated commit. It adds provenance information alongside Git history;
it does not replace code review, tests, or Git's own authorship metadata.

> Git can tell you which commit last changed a line. Auteur adds context about
> whether an agent was involved, which agent was reported, and what review
> evidence was recorded.

## What Auteur records

Auteur uses confidence levels to distinguish direct capture from inference:

| Confidence | Meaning |
|---|---|
| `high` | Captured by Auteur while an agent command or supported editor hook ran |
| `medium` | Inferred from a recognized `Co-authored-by` commit trailer |
| `low` | Reserved for future attribution methods |
| `unknown` | No reliable provenance record was found |

These levels describe the evidence available to Auteur. They are not a
cryptographic guarantee that a model generated particular text.

A provenance record can include:
- Changed file paths and line ranges.
- Agent name and, when supplied, model identifier.
- Session identifier and prompt hash, subject to privacy settings.
- Commit identity and timestamp.
- Human signoffs, including the signer's identity, public key, and SSH signature.

## Installation

Auteur requires Go 1.26 or later.

Install the latest published version:

```sh
go install github.com/stythelad/auteur/cmd/auteur@v0.5.4
```

Or build from source:

```sh
git clone https://github.com/stythelad/auteur.git
cd auteur
make install
```

Check the installation:

```sh
auteur --help
```

## Quick start

From inside an existing Git repository:

```sh
auteur init
```

Initialization creates or updates the following:

| Path | Purpose | Shared through Git? |
|---|---|---|
| `.git/auteur/provenance.jsonl` | Local provenance ledger | No |
| `.git/auteur/config.yaml` | Local Auteur configuration | No |
| `.auteur/policy.yaml` | Repository policy | Yes; commit it |
| `.claude/settings.json` | Merged Claude Code hooks, if used | Yes; review before committing |
| `.cursor/hooks.json` | Merged Cursor hooks, if used | Yes; review before committing |
| `.git/hooks/post-commit` | Resolves pending records after commits | No |
| `.git/hooks/post-rewrite` | Handles rewritten commits | No |

Initialization merges Auteur's Claude Code and Cursor hook entries into
existing editor configuration rather than replacing the whole file. Review
the resulting changes before committing them.

**Important:** the provenance ledger lives inside `.git/` and is local to a
clone. It is not part of the working tree and is not automatically shared with
other developers or CI runners. See [CI and shared provenance](#ci-and-shared-provenance).

## Capturing agent changes

### Claude Code

`auteur init` installs Claude Code hooks in `.claude/settings.json`.
The pre-edit hook snapshots files before an edit; the post-edit hook lets
Auteur compare the before and after states and record changed line ranges.

### Cursor

`auteur init` installs Cursor hooks in `.cursor/hooks.json`. Cursor's
available post-edit event does not provide the same before-edit snapshot,
so Auteur currently records file-level attribution for Cursor edits. This
can include human changes already present in the file; treat it as coarser
evidence than line-level capture.

### Other agents and command-line tools

Use `auteur wrap` to run a command and record the line changes it makes:

```sh
auteur wrap --agent codex --model gpt-example -- codex exec "fix the login bug"
git add .
git commit -m "fix login bug"
```

The wrapper records a pending provenance entry. The Git post-commit hook
associates it with the resulting commit.

You can also supply session metadata:

```sh
auteur wrap --agent copilot --session SESSION_ID -- copilot-cli "update tests"
```

See pending entries with:

```sh
auteur pending list
auteur pending show <uuid>
```

## Inspecting provenance

Show the recorded attribution for a line:

```sh
auteur blame auth/login.go:42
```

Use JSON output for tooling:

```sh
auteur blame --json auth/login.go:42
```

If Auteur has no matching record, it reports the origin as unknown. Absence
of a record should not be interpreted as proof that a human wrote the code.

## Bootstrapping existing history

Auteur can infer agent involvement from recognized commit trailers:

```sh
auteur scan-history --dry-run
auteur scan-history
```

History scanning is heuristic and records matches at `medium` confidence.
It does not reconstruct exact IDE sessions or prove that the trailer is
accurate.

Useful options include:

```sh
auteur scan-history --base HEAD --limit 500
auteur scan-history --include-merges
auteur scan-history --json
```

## Repository policy

The policy file is `.auteur/policy.yaml`. Auteur creates a default policy
during initialization if one does not already exist.

Example:

```yaml
version: 1
rules:
  - name: protected-paths-need-signoff
    paths:
      - "auth/**"
      - "payments/**"
      - "**/crypto/**"
    require_human_signoff: true
    require_provenance: true
    min_confidence: high

  - name: no-agent-code-in-migrations
    paths:
      - "migrations/**"
      - "**/migrations/**"
    forbid_agent_authored: true
    require_provenance: true
    min_confidence: high
```

Supported rule fields:

- `require_human_signoff`: agent-attributed changes need at least one
  recorded human signoff.
- `forbid_agent_authored`: agent-attributed changes are violations.
- `require_ci_passed`: agent-attributed changes need a recorded passing CI
  status.
- `require_provenance`: unknown-origin lines are violations.
- `min_confidence`: `high`, `medium`, or `low`; findings below this
  threshold are advisory rather than hard failures.

Each rule needs a name, at least one path pattern, and at least one enabled
requirement. Path patterns support glob matching, including `**` for
multiple path segments.

Run the policy locally:

```sh
auteur check
```

To check changes relative to a base branch:

```sh
git fetch origin
auteur check --ci-base origin/main
```

When `--ci-base` is used, Auteur reads the policy and trusted-signer list
from the specified base ref rather than trusting versions supplied by the
proposed changes. It also rejects changes to the trust-root file in the
compared changeset.

## Trusted signers and human approval

The trusted-signer file is `.auteur/allowed_signers`. Unlike the local
ledger, this file is intended to be committed to the repository so that
developers and CI can use the same trust root.

Add a public key:

```sh
auteur trust add "Alice" ~/.ssh/id_ed25519.pub
git add .auteur/allowed_signers
git commit -m "chore: add Alice as an Auteur trusted signer"
```

List or remove trusted signers:

```sh
auteur trust list
auteur trust remove "Alice"
```

Protect `.auteur/allowed_signers` with branch protections and
`CODEOWNERS`. Only trusted maintainers should be able to change it.

To approve a commit that has a provenance record:

```sh
auteur sign <commit-sha>
```

Auteur signs the provenance record with the configured SSH key and creates
an empty Git commit containing a signoff trailer. By default, it looks for
`~/.ssh/id_ed25519`; use `--ssh-key` to select another key.

For the accompanying Git commit signature, configure Git's SSH signing
support, for example:

```sh
git config gpg.format ssh
git config user.signingkey ~/.ssh/id_ed25519
```

The key must already be present in `.auteur/allowed_signers` under the
identity reported by `git config user.name`. Commit the updated trust-root
file before relying on it in CI.

Audit the ledger and signoffs:

```sh
auteur verify
```

Verification checks the ledger's hash chain and validates recorded SSH
signoffs against the configured trusted keys. It does not protect against
compromised signing keys, malicious maintainers, or a compromised machine.
Use it as one layer in a broader review and repository-security process.

## Privacy

Prompt capture is **off by default**. The default local configuration is
stored in `.git/auteur/config.yaml`:

```yaml
version: 1
privacy:
  capture_prompt: false
  prompt_max_length: 200
  redact_secrets: true
  capture_session_id: true
```

- Set `capture_prompt: true` to opt in to storing prompt text.
- Set `capture_prompt: false` to avoid storing prompt text; Auteur can still
  retain a prompt hash where available.
- `prompt_max_length` limits captured prompt length.
- `redact_secrets` enables secret-pattern redaction.
- `capture_session_id` controls whether session identifiers are recorded.

Review your organization's data-handling requirements before enabling prompt
capture. Redaction is a safeguard, not a guarantee that sensitive information
will never be captured.

## CI and shared provenance

The repository includes a composite GitHub Action:

```yaml
name: Auteur
on:
  pull_request:

jobs:
  provenance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: stythelad/auteur@v0.5.4
        with:
          base-ref: ${{ github.base_ref }}
```

The action installs Auteur and runs `auteur check --ci-base` against the
base branch. `fetch-depth: 0` makes the relevant Git history available.

**CI limitation:** the action does not transfer or reconstruct the local
`.git/auteur/provenance.jsonl` ledger. Because that ledger is not committed,
a fresh GitHub Actions checkout will not automatically have the provenance
records created on a developer's machine. The ledger must be made available
to the runner by a separate, trusted process before this check can enforce
record-level policy. Do not assume that adding the action alone creates a
complete end-to-end provenance gate.

The trust root and policy are read from the base branch when using
`--ci-base`; the provenance ledger remains a separate input.

## Commands

| Command | Purpose |
|---|---|
| `auteur init` | Initialize local ledger, config, policy, and hooks |
| `auteur wrap -- <command>` | Record changes made by a command |
| `auteur blame <file>:<line>` | Show provenance for a line |
| `auteur pending list` | List unresolved pending records |
| `auteur pending show <uuid>` | Inspect a pending record |
| `auteur pending gc` | Remove stale pending records |
| `auteur resolve-pending` | Resolve pending records against Git history |
| `auteur scan-history` | Infer attribution from commit trailers |
| `auteur check` | Evaluate repository policy |
| `auteur sign <sha>` | Sign off on a recorded commit |
| `auteur trust add/list/remove` | Manage trusted signers |
| `auteur verify` | Verify ledger integrity and signoffs |
| `auteur rewrite <old> <new>` | Copy attribution to rewritten commit SHAs |

Run `auteur <command> --help` for command-specific options.

## Design notes

### Why is the ledger stored in `.git/auteur/`?

The ledger is local to a clone and shared by linked Git worktrees through the
common Git directory. It avoids adding generated provenance data to the
working tree, but it also means the ledger is not automatically shared with
other clones or CI.

### Why JSONL?

The ledger uses one JSON record per line. This makes appending records
straightforward and allows individual records to be inspected with ordinary
text tools.

### Why confidence levels?

Attribution sources have different strengths. Direct capture, commit-trailer
inference, and missing evidence should not be presented as equally reliable.

## Current status

Auteur is early-stage software. Its interfaces and ledger format may change
before v1.0. Review the threat model and test the workflow in a non-critical
repository before making it part of a security-sensitive release process.

## Contributing

Issues and pull requests are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
