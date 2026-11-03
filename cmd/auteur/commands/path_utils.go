package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

func safeRelPath(root, target string) (string, error) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path %q is outside the repository", target)
	}
	return rel, nil
}

func snapshotPath(snapDir, relPath string) string {
	hash := sha256.Sum256([]byte(relPath))
	return filepath.Join(snapDir, hex.EncodeToString(hash[:8])+".snap")
}
