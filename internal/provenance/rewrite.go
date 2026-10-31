package provenance

import (
	"fmt"
	"strings"
)

// RewriteRecords copies records for each old SHA to a new record with the
// new SHA. Used by the post-rewrite hook to keep attribution alive across
// rebase and commit --amend.
//
// Old records are left in place: blame walks records in reverse, so the
// newest (new-SHA) record wins for the rewritten commit, and the old
// record still answers queries against the pre-rewrite commit.
//
// Returns the number of records copied.
func RewriteRecords(repoRoot string, pairs map[string]string) (int, error) {
	if len(pairs) == 0 {
		return 0, nil
	}
	store := Open(repoRoot)
	recs, err := store.Load()
	if err != nil {
		return 0, err
	}

	// Group existing records by their commit SHA.
	bySHA := map[string][]Record{}
	for _, r := range recs {
		bySHA[r.Commit] = append(bySHA[r.Commit], r)
	}

	copied := 0
	for oldSHA, newSHA := range pairs {
		if oldSHA == newSHA {
			continue
		}
		for _, r := range bySHA[oldSHA] {
			c := r
			c.Commit = newSHA
			if err := store.Append(c); err != nil {
				return copied, fmt.Errorf("append rewritten record: %w", err)
			}
			copied++
		}
	}
	return copied, nil
}

// ParseRewritePairs parses "old new\nold new\n..." input, as delivered by
// git's post-rewrite hook on stdin.
func ParseRewritePairs(input string) map[string]string {
	pairs := map[string]string{}
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		pairs[fields[0]] = fields[1]
	}
	return pairs
}
