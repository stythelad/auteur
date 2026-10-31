package commands

import (
	"fmt"
	"os"
	"path/filepath"
)

const postCommitHook = `#!/bin/sh
if command -v auteur >/dev/null 2>&1; then
  auteur resolve-pending >/dev/null 2>&1 || true
fi
`

// installPostCommitHook writes the hook into .git/hooks/post-commit
// relative to the given repo root. Idempotent: skips if an identical
// hook is already present, errors if a different hook is.
func installPostCommitHook(repoRoot string, force bool) (installed bool, err error) {
	hookPath := filepath.Join(repoRoot, ".git", "hooks", "post-commit")
	existing, readErr := os.ReadFile(hookPath)
	if readErr == nil {
		if string(existing) == postCommitHook {
			return false, nil
		}
		if !force {
			return false, fmt.Errorf("existing post-commit hook differs; use --force to overwrite")
		}
	}
	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return false, fmt.Errorf("create hooks dir: %w", err)
	}
	if err := os.WriteFile(hookPath, []byte(postCommitHook), 0o755); err != nil {
		return false, fmt.Errorf("write hook: %w", err)
	}
	return true, nil
}
