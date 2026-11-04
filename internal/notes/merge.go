package notes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

// FetchAndMerge fetches notes from the remote and merges them into the local notes ref.
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
// It returns errors instead of swallowing them.
func readRecordsFromRef(repoRoot, ref string) ([]provenance.Record, error) {
	refName := strings.TrimPrefix(ref, "refs/notes/")
	out, err := git.Run(repoRoot, "notes", "--ref="+refName, "list")
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", ref, err)
	}

	var all []provenance.Record
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) != 2 {
			continue
		}
		objSHA := parts[1]
		raw, err := git.Run(repoRoot, "notes", "--ref="+refName, "show", objSHA)
		if err != nil {
			return nil, fmt.Errorf("show %s: %w", objSHA[:7], err)
		}
		recs, err := parseNotesJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("parse note on %s: %w", objSHA[:7], err)
		}
		all = append(all, recs...)
	}
	return all, nil
}

// parseNotesJSON parses either a signed envelope, an array of records, or a single record.
func parseNotesJSON(raw string) ([]provenance.Record, error) {
	// Try envelope first
	var env Envelope
	if err := json.Unmarshal([]byte(raw), &env); err == nil && env.Signature != "" {
		return env.Records, nil
	}

	// Try array
	var recs []provenance.Record
	if err := json.Unmarshal([]byte(raw), &recs); err == nil {
		return recs, nil
	}

	// Try single object
	var single provenance.Record
	if err := json.Unmarshal([]byte(raw), &single); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return []provenance.Record{single}, nil
}

// recordKey generates a unique key for deduplication, including signoffs.
func recordKey(r provenance.Record) string {
	key := r.Commit + "|" + string(r.Confidence)
	if r.Agent != nil {
		key += "|" + r.Agent.Name
	}
	for _, lr := range r.Lines {
		key += fmt.Sprintf("|%s:%d-%d", lr.File, lr.Start, lr.End)
	}
	for _, s := range r.Review.Signoffs {
		key += "|signoff:" + s.Identity
	}
	return key
}
