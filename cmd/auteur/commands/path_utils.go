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
func safeRelPath(root, target string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("could not resolve repo root: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return "", err
	}

	if realTarget, err := filepath.EvalSymlinks(target); err == nil {
		realTarget, _ = filepath.Abs(realTarget)
		return checkRel(realRoot, realTarget)
	}

	dir := filepath.Dir(target)
	base := filepath.Base(target)
	var missingComponents []string
	var realDir string

	for {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			realDir, err = filepath.Abs(resolved)
			if err != nil {
				return "", err
			}
			break
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("could not resolve path %q: %w", dir, err)
		}

		parent := filepath.Base(dir)
		if parent == string(filepath.Separator) || parent == "." || parent == ".." {
			return "", fmt.Errorf("could not find any existing parent for %q", target)
		}
		missingComponents = append([]string{parent}, missingComponents...)
		dir = filepath.Dir(dir)
	}

	parts := append(missingComponents, base)
	realTarget := filepath.Join(realDir, filepath.Join(parts...))
	return checkRel(realRoot, realTarget)
}

func checkRel(realRoot, realTarget string) (string, error) {
	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil {
		return "", fmt.Errorf("path %q is on a different volume or invalid: %w", realTarget, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside the repository", realTarget)
	}
	return realTarget, nil
}

func snapshotPath(snapDir, relPath string) string {
	hash := sha256.Sum256([]byte(relPath))
	return filepath.Join(snapDir, hex.EncodeToString(hash[:8])+".snap")
}
