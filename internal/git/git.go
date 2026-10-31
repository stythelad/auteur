package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes git with args in dir and returns trimmed stdout.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// RepoRoot returns the absolute path of the git repository containing dir.
func RepoRoot(dir string) (string, error) {
	return Run(dir, "rev-parse", "--show-toplevel")
}

// HeadSHA returns the current HEAD commit SHA.
func HeadSHA(dir string) (string, error) {
	return Run(dir, "rev-parse", "HEAD")
}

// IsRepo reports whether dir is inside a git work tree.
func IsRepo(dir string) bool {
	_, err := Run(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil
}
