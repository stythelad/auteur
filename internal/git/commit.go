package git

import (
	"strings"
)

// CommitFiles returns the set of paths changed by the given commit vs its parent.
// If the commit has no parent (root commit), it diffs against the empty tree.
func CommitFiles(dir, sha string) (map[string]bool, error) {
	parents, err := Run(dir, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(parents)
	var diff string
	if len(parts) <= 1 {
		emptyTree, eErr := Run(dir, "hash-object", "-t", "tree", "/dev/null")
		if eErr != nil {
			return nil, eErr
		}
		diff, err = Run(dir, "diff", "--name-only", emptyTree, sha)
	} else {
		diff, err = Run(dir, "diff", "--name-only", parts[1], sha)
	}
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, line := range strings.Split(diff, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out[line] = true
		}
	}
	return out, nil
}

// CommitTime returns the commit's author timestamp in RFC3339.
func CommitTime(dir, sha string) (string, error) {
	return Run(dir, "show", "-s", "--format=%aI", sha)
}
