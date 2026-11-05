# Auteur

**Provenance tracking and policy checks for AI-assisted code.**

![Auteur demo](docs/demo.gif)

Git records who committed a change. Auteur records additional context about
which lines were attributed to an AI agent, which tool or model was involved,
and whether a human reviewer signed off.

```text
$ auteur blame auth/login.py:42
auth/login.py:42
  author:  claude-code (claude-opus-5.5)
  session: sess_abc123
  commit:  62e291c  (2026-10-31)
  range:   40-55
  origin:  agent   [confidence: high]
```

Auteur is designed to make provenance easier to inspect and policy-check. It
does not independently prove that a model produced particular code: attribution
depends on the capture mechanism, the integrity of the repository and notes,
and the trust placed in signing keys and contributors. Treat provenance as
auditable evidence, not a substitute for code review. `auteur blame` is a
best-effort lookup: its current line-to-record matching does not prove that the
specific displayed line was introduced by the recorded author.

## What Auteur records

Auteur associates provenance records with Git commits. Records can include:

- File paths and changed line ranges.
- Origin classification, such as agent-authored.
- Agent name, model, and session identifier when available.
- Confidence level.
- Human signoffs and cryptographic signature metadata.

Confidence levels:

| Level | Meaning |
|---|---|
| `high` | Captured directly by Auteur while an agent command or supported editor hook runs. |
| `medium` | Inferred from recognized `Co-authored-by` commit trailers by `auteur scan-history`. |
| `low` | Reserved for lower-confidence heuristics. |
| Unknown | No matching provenance record was found. Absence of a record is not proof that a human wrote the code. |

## Install

Auteur requires **Go 1.26** to build from source.

Install the v0.6.5 release:

```sh
go install github.com/stythelad/auteur/cmd/auteur@v0.6.5
```

Or build from a checkout:

```sh
git clone https://github.com/stythelad/auteur.git
cd auteur
go build -o auteur ./cmd/auteur
```

Make sure the installed `auteur` binary is on your `PATH`.

## Quick start

From inside an existing Git repository:

```sh
auteur init
```

Initialization creates or initializes:

| Path | Purpose | Shared with Git? |
|---|---|---|
| `.git/auteur/provenance.jsonl` | Local provenance ledger/cache | No |
| `.git/auteur/config.yaml` | Local Auteur settings | No |
| `.auteur/policy.yaml` | Repository policy | Yes; commit it |
| `.git/hooks/post-commit` | Resolves pending records after commits | Local |
| `.git/hooks/post-rewrite` | Preserves records across rewritten commits | Local |
| `.claude/settings.json` | Claude Code hook configuration, merged if present | Usually commit if you share project settings |
| `.cursor/hooks.json` | Cursor hook configuration, merged if present | Usually commit if you share project settings |

The trusted signer list is stored separately in `.auteur/allowed_signers`.
Add it with `auteur trust add` and commit it to the repository.

## Capture agent changes

### Claude Code and Cursor hooks

`auteur init` merges Auteur hooks into the supported editor configuration
files. Claude Code supports pre-edit and post-edit capture, allowing Auteur
to compare the file before and after an edit. Cursor's available post-edit
hook does not provide the same before-edit snapshot; Cursor attribution can
therefore be coarser and may include other edits already present in the file.

Review the generated hook configuration and test it with your installed
versions of the editors. Hook availability and event behavior can change
between editor releases.

### Wrap another agent or command

For tools without a supported lifecycle hook, run the command through
`auteur wrap`:

```sh
auteur wrap --agent codex --model gpt-example -- codex "fix the login bug"
git add .
git commit -m "fix login bug"
```

Auteur compares repository snapshots before and after the wrapped command and
stores a pending record. The post-commit hook resolves pending records to the
new commit. You can also inspect pending work:

```sh
auteur pending list
auteur pending show <uuid>
```

If a pending record is stale, remove old pending records with:

```sh
auteur pending gc --older-than 7d
```

### Import existing history

Scan commit messages for recognized agent `Co-authored-by` trailers:

```sh
auteur scan-history --dry-run
auteur scan-history
```

History scanning infers attribution and writes records with **medium**
confidence. It is not equivalent to capture at the time the code was edited.

## Shared provenance with Git notes

Auteur mirrors provenance records to the Git notes ref:

```text
refs/notes/auteur
```

Git notes can be transferred between clones independently of ordinary commit
history. Use the commands below to synchronize them:

```sh
auteur fetch
auteur push
```

The GitHub Action described below fetches the remote Auteur notes before
running its policy check. If you use a different CI system, fetch the notes
before invoking `auteur check`.

Notes are associated with commit object IDs. If a hosting platform squash-merges
a pull request and replaces the original commits, use `auteur resolve-squash`
to copy attribution to the resulting squash commit. The command supports
explicit original commit IDs and `Auteur-Origin-SHA:` trailers; see:

```sh
auteur resolve-squash --help
```

For rebases and amended commits, Auteur installs a post-rewrite hook and also
provides:

```sh
auteur rewrite --help
```

Keep notes synchronized with the remote so that other clones and CI can see
the records they need.

## Configure policy

The repository policy lives in `.auteur/policy.yaml`. `auteur init` creates a
default policy if one does not already exist.

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

  - name: require-ci-attestation
    paths:
      - "auth/**"
    require_ci_passed: true
```

Supported rule fields:

- `paths`: path patterns to which the rule applies.
- `require_human_signoff`: require a trusted human signoff.
- `forbid_agent_authored`: disallow agent-origin records.
- `require_ci_passed`: require the CI-passed condition.
- `require_provenance`: require provenance evidence.
- `min_confidence`: `high`, `medium`, or `low`.

Each rule must set at least one of the four boolean requirements. Path patterns
support `*`, `?`, and `**` for multi-segment matching.

Run a local check:

```sh
auteur check
```

For pull-request checks, compare against the base branch:

```sh
git fetch origin
auteur check --ci-base origin/main
```

The `--ci-base` option reads the policy and trusted signer list from the
specified base ref, rather than trusting versions introduced by the pull
request. It also rejects changes to the committed trust-root file in the PR.

Other useful options:

```sh
auteur check --since origin/main
auteur check --pr-base main
auteur check --json
auteur check --no-strict
```

Review the check output and CI exit status together. As with any provenance
system, policy results are only as reliable as the records and validation
available to that version of Auteur.

## Trusted signers and approvals

The committed trust root is:

```text
.auteur/allowed_signers
```

Add a signer's public SSH key:

```sh
auteur trust add "Alice" ~/.ssh/id_ed25519.pub
auteur trust list
```

Commit `.auteur/allowed_signers` and protect changes to it with your normal
repository controls, such as CODEOWNERS and required reviews. Remove a signer
with `auteur trust remove "Alice"`.

To sign off on a provenance record:

```sh
auteur sign <commit-sha>
```

You can specify a private key with `--ssh-key`. The default is
`~/.ssh/id_ed25519`. The signer identity is taken from `git config user.name`,
and the corresponding public key must be present in the trust root.

The sign command also creates an empty SSH-signed Git commit containing an
Auteur signoff trailer. Configure Git SSH commit signing (including
`gpg.format=ssh` and `user.signingkey`) before using this command.

Audit the locally available provenance records with:

```sh
auteur verify
```

A successful audit is not a guarantee against compromised signing keys,
malicious trusted contributors, or defects in the capture and verification
implementation. Keep normal code review and branch protections in place.

## Privacy

Auteur's local settings are stored in `.git/auteur/config.yaml`. The default
configuration does **not** capture prompt text. Prompt capture is opt-in.
Secret redaction is enabled by default, and session IDs can be captured.

Example settings:

```yaml
version: 1
privacy:
  capture_prompt: false
  prompt_max_length: 200
  redact_secrets: true
  capture_session_id: true
```

Review your local configuration and your organization's data-handling
requirements before enabling prompt capture. Redaction is a safeguard, not a
guarantee that all sensitive information will be detected.

## GitHub Actions

The repository provides a composite action at the repository root. Example
workflow:

```yaml
name: Auteur
on:
  pull_request:

jobs:
  provenance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 0

      - uses: stythelad/auteur@v0.6.5
        with:
          base-ref: ${{ github.base_ref }}
          auteur-version: v0.6.5
```

The action:

1. Sets up Go 1.26.
2. Installs the requested Auteur version.
3. Attempts to fetch `refs/notes/auteur` from `origin`.
4. Runs `auteur check --ci-base origin/<base-ref>`.

Use a full checkout so the base ref and relevant commit history are available.
The action's `base-ref` input is required; `auteur-version` defaults to
`v0.6.5`.

The action treats a failed notes fetch as non-fatal and continues to the check.
Ensure your repository actually publishes the notes ref and that the workflow
has the Git permissions and remote configuration needed to fetch it. A missing
notes ref is not evidence that all changed code has valid provenance.

## Command reference

| Command | Purpose |
|---|---|
| `auteur init` | Initialize local state and install Git/editor hooks |
| `auteur wrap -- <command>` | Capture changes made by a wrapped command |
| `auteur blame <file>:<line>` | Show candidate provenance associated with a line (best-effort; not proof of authorship) |
| `auteur check` | Evaluate provenance policy |
| `auteur sign [commit-sha]` | Sign off on a commit's provenance |
| `auteur verify` | Audit local provenance integrity and signatures |
| `auteur trust add/list/remove` | Manage trusted signers |
| `auteur scan-history` | Infer records from commit trailers |
| `auteur push` / `auteur fetch` | Synchronize Git notes |
| `auteur resolve-squash` | Copy provenance to a squash-merge commit |
| `auteur rewrite` | Preserve records across rewritten commits |
| `auteur pending list/show/gc` | Inspect or prune pending records |
| `auteur resolve-pending` | Resolve pending records to HEAD |

Run `auteur <command> --help` for flags and detailed usage.

## Current status

Auteur is pre-1.0 software. Interfaces and record formats may change before
1.0. The current release is v0.6.5. Review the changelog for release-specific
changes and known limitations before adopting it in a security-critical
workflow.

## Contributing

Issues and pull requests are welcome. Run the test suite and static checks
before submitting changes:

```sh
go test ./...
go vet ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT. See [LICENSE](LICENSE).
