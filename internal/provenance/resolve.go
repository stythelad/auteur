package provenance

import (
	"fmt"
	"strings"
)

// ResolvePending walks every pending record and, for any whose lines
// intersect changedFiles, appends a resolved Record to the ledger with
// the given commit SHA and deletes the pending file.
//
// Returns the number of records resolved.
func ResolvePending(repoRoot, commitSHA, commitTimestamp string, changedFiles map[string]bool) (int, error) {
	pendings, err := ListPending(repoRoot)
	if err != nil {
		return 0, err
	}
	if len(pendings) == 0 {
		return 0, nil
	}
	store := Open(repoRoot)
	resolved := 0
	for _, p := range pendings {
		if !touchesAny(p.Record.Lines, changedFiles) {
			continue
		}
		rec := p.Record
		rec.Commit = commitSHA
		if commitTimestamp != "" {
			rec.Timestamp = parseOrNow(commitTimestamp)
		}
		if err := store.Append(rec); err != nil {
			return resolved, fmt.Errorf("append resolved record: %w", err)
		}
		if err := DeletePending(repoRoot, p.UUID); err != nil {
			return resolved, err
		}
		resolved++
	}
	return resolved, nil
}

func touchesAny(ranges []LineRange, changed map[string]bool) bool {
	for _, r := range ranges {
		if changed[r.File] {
			return true
		}
		if changed[strings.TrimPrefix(r.File, "./")] {
			return true
		}
	}
	return false
}
