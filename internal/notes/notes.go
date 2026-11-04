package notes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

const NotesRef = "auteur"

// WriteRecord appends a provenance Record to the Git note for a commit SHA.
func WriteRecord(repoRoot, sha string, r provenance.Record) error {
	existing, found, err := ReadRecords(repoRoot, sha)
	if err != nil {
		return err
	}

	var records []provenance.Record
	if found {
		records = existing
	}

	// Avoid duplicates (same commit, lines, and origin)
	// For simplicity, we just append. The merge strategy will dedup later.
	records = append(records, r)

	b, err := json.Marshal(records)
	if err != nil {
		return fmt.Errorf("marshal records: %w", err)
	}

	out, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "add", "-f", "-m", string(b), sha)
	if err != nil {
		return fmt.Errorf("git notes add: %w\n%s", err, out)
	}
	return nil
}

// ReadRecords retrieves all provenance Records for a specific commit SHA.
func ReadRecords(repoRoot, sha string) ([]provenance.Record, bool, error) {
	out, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "show", sha)
	if err != nil {
		if strings.Contains(err.Error(), "no note found") {
			return nil, false, nil
		}
		return nil, false, err
	}

	// Try parsing as array first
	var records []provenance.Record
	if err := json.Unmarshal([]byte(out), &records); err == nil {
		return records, true, nil
	}

	// Fallback to single object (backward compatibility)
	var single provenance.Record
	if err := json.Unmarshal([]byte(out), &single); err == nil {
		return []provenance.Record{single}, true, nil
	}

	return nil, false, fmt.Errorf("failed to parse note: %s", out)
}

// ListAllRecords returns all provenance records in the repo.
func ListAllRecords(repoRoot string) ([]provenance.Record, error) {
	out, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "list")
	if err != nil {
		if strings.Contains(err.Error(), "no notes found") {
			return nil, nil
		}
		return nil, err
	}

	var allRecords []provenance.Record
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		objSha := parts[1]

		recs, found, err := ReadRecords(repoRoot, objSha)
		if err != nil {
			return nil, err
		}
		if found {
			allRecords = append(allRecords, recs...)
		}
	}
	return allRecords, nil
}

func Push(repoRoot, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := git.Run(repoRoot, "push", remote, "refs/notes/"+NotesRef)
	return err
}

func Fetch(repoRoot, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := git.Run(repoRoot, "fetch", remote, fmt.Sprintf("refs/notes/%s:refs/notes/%s", NotesRef, NotesRef))
	return err
}
