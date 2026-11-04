package notes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

// FetchAndMerge fetches notes from the remote and merges them into the local notes ref.
// It handles diverged histories by union-merging records.
func FetchAndMerge(repoRoot, remote string) error {
	if remote == "" {
		remote = "origin"
	}

	tmpRef := "refs/notes/auteur-remote"
	_, err := git.Run(repoRoot, "fetch", remote, fmt.Sprintf("refs/notes/%s:%s", NotesRef, tmpRef))
	if err != nil {
		if strings.Contains(err.Error(), "couldn't find remote ref") || strings.Contains(err.Error(), "no such ref") {
			return nil
		}
		return fmt.Errorf("fetch notes: %w", err)
	}

	localRecs, _ := ListAllRecords(repoRoot)
	remoteRecs, err := readRecordsFromRef(repoRoot, tmpRef)
	if err != nil {
		return fmt.Errorf("read remote notes: %w", err)
	}

	if len(remoteRecs) == 0 {
		return nil
	}

	seen := map[string]bool{}
	for _, r := range localRecs {
		seen[recordKey(r)] = true
	}

	for _, r := range remoteRecs {
		key := recordKey(r)
		if !seen[key] {
			if err := WriteRecord(repoRoot, r.Commit, r); err != nil {
				return fmt.Errorf("write merged note: %w", err)
			}
			seen[key] = true
		}
	}
	return nil
}

// readRecordsFromRef reads all records from a specific notes ref.
func readRecordsFromRef(repoRoot, ref string) ([]provenance.Record, error) {
	out, err := git.Run(repoRoot, "notes", "--ref="+ref, "list")
	if err != nil {
		return nil, nil
	}

	var all []provenance.Record
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		objSHA := parts[1]
		noteOut, err := git.Run(repoRoot, "notes", "--ref="+ref, "show", objSHA)
		if err != nil {
			continue
		}
		recs := parseNotesJSON(noteOut)
		all = append(all, recs...)
	}
	return all, nil
}

// parseNotesJSON handles both array and single-object formats.
func parseNotesJSON(raw string) []provenance.Record {
	// Try array
	var recs []provenance.Record
	if err := json.Unmarshal([]byte(raw), &recs); err == nil {
		return recs
	}
	// Try single object
	var single provenance.Record
	if err := json.Unmarshal([]byte(raw), &single); err == nil {
		return []provenance.Record{single}
	}
	return nil
}

// recordKey generates a unique key for deduplication.
func recordKey(r provenance.Record) string {
	key := r.Commit + "|" + string(r.Confidence)
	if r.Agent != nil {
		key += "|" + r.Agent.Name
	}
	for _, lr := range r.Lines {
		key += fmt.Sprintf("|%s:%d-%d", lr.File, lr.Start, lr.End)
	}
	return key
}
