package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

const postCommitHook = `#!/bin/sh
if command -v auteur >/dev/null 2>&1; then
  auteur resolve-pending >/dev/null 2>&1 || true
  auteur resolve-squash --from-message >/dev/null 2>&1 || true
fi
`

const postRewriteHook = `#!/bin/sh
if command -v auteur >/dev/null 2>&1; then
  auteur rewrite --stdin >/dev/null 2>&1 || true
fi
`

// installHooks writes the post-commit and post-rewrite hooks into .git/hooks.
// Idempotent: skips hooks that already match, errors on conflicting hooks
// unless force is set. Returns (postCommitInstalled, postRewriteInstalled, error).
func installHooks(repoRoot string, force bool) (bool, bool, error) {
	pc, err := installOneHook(repoRoot, "post-commit", postCommitHook, force)
	if err != nil {
		return false, false, err
	}
	pr, err := installOneHook(repoRoot, "post-rewrite", postRewriteHook, force)
	if err != nil {
		return pc, false, err
	}
	return pc, pr, nil
}

func installOneHook(repoRoot, name, content string, force bool) (installed bool, err error) {
	path := filepath.Join(repoRoot, ".git", "hooks", name)
	existing, readErr := os.ReadFile(path)
	if readErr == nil {
		if string(existing) == content {
			return false, nil
		}
		if !force {
			return false, fmt.Errorf("existing %s hook differs; use --force to overwrite", name)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create hooks dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return false, fmt.Errorf("write %s hook: %w", name, err)
	}
	return true, nil
}
