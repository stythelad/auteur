package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// safeRelPath resolves the target path and ensures it is strictly inside the repository root.
// It returns the canonical, resolved path to be used for file access.
func safeRelPath(root, target string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("could not resolve repo root: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return "", err
	}

	// Try to resolve the full path first (handles symlinked files)
	if realTarget, err := filepath.EvalSymlinks(target); err == nil {
		realTarget, _ = filepath.Abs(realTarget)
		return checkRel(realRoot, realTarget)
	}

	// Target doesn't exist. Resolve the deepest existing parent.
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

	// Reconstruct: realDir + missing dirs + base filename
	parts := append(missingComponents, base)
	realTarget := filepath.Join(realDir, filepath.Join(parts...))
	return checkRel(realRoot, realTarget)
}

func checkRel(realRoot, realTarget string) (string, error) {
	rel, err := filepath.Rel(realRoot, realTarget)
	if err != nil {
		return "", fmt.Errorf("path %q is on a different volume or invalid: %w", realTarget, err)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside the repository", realTarget)
	}
	return realTarget, nil // Return the resolved absolute path, not the relative one
}

// secureReadFile opens a file with O_NOFOLLOW to prevent TOCTOU symlink attacks.
// It returns the file contents. If the file does not exist, it returns an empty slice.
func secureReadFile(path string) ([]byte, error) {
	// O_NOFOLLOW ensures the *final* component is not a symlink.
	// We rely on safeRelPath to have resolved parent directories safely.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if os.IsNotExist(err) {
			// Distinguish between a normal missing file and a dangling symlink.
			// If Lstat succeeds, the path exists as a symlink (or file), but the target is missing.
			if info, lerr := os.Lstat(path); lerr == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf("refusing to read dangling symlink at %s", path)
				}
			}
			return []byte{}, nil // Ordinary new file
		}
		// If it's a symlink, ELOOP is returned (even if the target is dangling).
		if errors.Is(err, syscall.ELOOP) {
			return nil, fmt.Errorf("refusing to follow symlink at %s", path)
		}
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

func snapshotPath(snapDir, relPath string) string {
	hash := sha256.Sum256([]byte(relPath))
	return filepath.Join(snapDir, hex.EncodeToString(hash[:8])+".snap")
}
