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
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside the repository", realTarget)
	}
	return realTarget, nil
}

// secureReadFile opens a file with O_NOFOLLOW to prevent TOCTOU symlink attacks.
// It also re-validates the parent directory immediately before opening to prevent
// parent-directory replacement attacks.
func secureReadFile(root, path string) ([]byte, error) {
	// Re-validate the path immediately before reading.
	// This drastically narrows the window for a TOCTOU attack.
	revalidatedPath, err := safeRelPath(root, path)
	if err != nil {
		return nil, fmt.Errorf("revalidation failed: %w", err)
	}
	if revalidatedPath != path {
		return nil, fmt.Errorf("path changed during validation: expected %s, got %s", path, revalidatedPath)
	}

	// O_NOFOLLOW ensures the *final* component is not a symlink.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		if os.IsNotExist(err) {
			if info, lerr := os.Lstat(path); lerr == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					return nil, fmt.Errorf("refusing to read dangling symlink at %s", path)
				}
			}
			return []byte{}, nil
		}
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
