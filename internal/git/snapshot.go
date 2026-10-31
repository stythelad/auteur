package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func StashCreate(dir string) (string, error) {
	return Run(dir, "stash", "create")
}

func UntrackedFiles(dir string) ([]string, error) {
	out, err := Run(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func DiffFrom(dir, fromSHA string) (string, error) {
	return Run(dir, "diff", fromSHA, "--")
}

// RunAllowExit1 executes git and returns stdout even if the command exits 1.
// Useful for `git diff --no-index`, which exits 1 when files differ (normal).
// Any exit code other than 0 or 1 is treated as an error.
func RunAllowExit1(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return out.String(), nil
		}
		return out.String(), fmt.Errorf("git %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return out.String(), nil
}
