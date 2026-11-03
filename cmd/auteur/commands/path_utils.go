package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// safeRelPath returns a relative path if it is strictly inside the root,
// resolving symlinks to prevent traversal attacks.
func safeRelPath(root, target string) (string, error) {
	// 1. Resolve the root to its canonical path
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}

	// 2. Resolve the target's parent directory
	dir := filepath.Dir(target)
	base := filepath.Base(target)

	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		// If the directory doesn't exist, it's a new file in a non-existent dir.
		// We check if the lexical path is safe. If a user tries to create
		// files in non-existent directories outside the repo, lexical check catches it.
		realDir = dir
	}

	realTarget := filepath.Join(realDir, base)

	// 3. Compute relative path from realRoot to realTarget
	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil {
		return "", err
	}

	// 4. Check if it escaped the root
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside the repository", target)
	}

	return rel, nil
}

func snapshotPath(snapDir, relPath string) string {
	hash := sha256.Sum256([]byte(relPath))
	return filepath.Join(snapDir, hex.EncodeToString(hash[:8])+".snap")
}
