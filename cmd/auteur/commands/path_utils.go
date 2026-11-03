package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// safeRelPath resolves the target path and ensures it is strictly inside the repository root.
// It handles symlinks, non-existent files, and non-existent parent directories safely.
func safeRelPath(root, target string) (string, error) {
	// 1. Resolve the repo root to its canonical path.
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("could not resolve repo root: %w", err)
	}

	// Ensure the root itself is absolute
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return "", err
	}

	// 2. If the target itself exists (or is a symlink), resolve it fully.
	if realTarget, err := filepath.EvalSymlinks(target); err == nil {
		realTarget, _ = filepath.Abs(realTarget)
		return checkRel(realRoot, realTarget)
	}

	// 3. The target doesn't exist. Find the deepest existing parent.
	dir := filepath.Dir(target)
	var missingComponents []string
	var realDir string

	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			// We found the deepest existing directory.
			// It must resolve successfully and be absolute.
			realDir, err = filepath.Abs(resolved)
			if err != nil {
				return "", err
			}
			break
		} else if !os.IsNotExist(err) {
			// If the error is NOT "file not found" (e.g., permission denied), fail safely.
			return "", fmt.Errorf("could not resolve path %q: %w", dir, err)
		}

		// This directory doesn't exist. Move up one level.
		base := filepath.Base(dir)
		if base == string(filepath.Separator) || base == "." || base == ".." {
			return "", fmt.Errorf("could not find any existing parent for %q", target)
		}
		missingComponents = append([]string{base}, missingComponents...)
		dir = filepath.Dir(dir)
	}

	// 4. Reconstruct the target using the resolved parent and the missing components.
	realTarget := filepath.Join(realDir, filepath.Join(missingComponents...))

	// Final check
	return checkRel(realRoot, realTarget)
}

// checkRel ensures realTarget is strictly inside realRoot.
func checkRel(realRoot, realTarget string) (string, error) {
	// Use filepath.Rel to compute the relative path.
	// If they are on different drives (Windows), this returns an error.
	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil {
		return "", fmt.Errorf("path %q is on a different volume or invalid: %w", realTarget, err)
	}

	// If the relative path starts with "..", it's outside the root.
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside the repository", realTarget)
	}

	// Normalize path separators for the return value (always use forward slashes for consistency)
	return filepath.ToSlash(rel), nil
}

func snapshotPath(snapDir, relPath string) string {
	hash := sha256.Sum256([]byte(relPath))
	return filepath.Join(snapDir, hex.EncodeToString(hash[:8])+".snap")
}
