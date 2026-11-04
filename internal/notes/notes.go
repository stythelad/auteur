package notes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

const NotesRef = "auteur"

// WriteRecord attaches a provenance Record to a specific commit SHA as a Git note.
func WriteRecord(repoRoot, sha string, r provenance.Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	// git notes add -f -m <json> <sha>
	// We use -f to overwrite if a note already exists (e.g., for updates)
	out, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "add", "-f", "-m", string(b), sha)
	if err != nil {
		return fmt.Errorf("git notes add: %w\n%s", err, out)
	}
	return nil
}

// ReadRecord retrieves the provenance Record for a specific commit SHA.
func ReadRecord(repoRoot, sha string) (provenance.Record, bool, error) {
	out, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "show", sha)
	if err != nil {
		// If the commit has no note, git returns an error. We treat this as "not found".
		if strings.Contains(err.Error(), "no note found") {
			return provenance.Record{}, false, nil
		}
		return provenance.Record{}, false, err
	}

	var r provenance.Record
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return provenance.Record{}, false, fmt.Errorf("unmarshal note: %w", err)
	}
	return r, true, nil
}

// ListAllRecords returns all provenance records in the repo.
func ListAllRecords(repoRoot string) ([]provenance.Record, error) {
	// git notes list returns lines of "<note-sha> <object-sha>"
	out, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "list")
	if err != nil {
		if strings.Contains(err.Error(), "no notes found") {
			return nil, nil
		}
		return nil, err
	}

	var records []provenance.Record
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		objSha := parts[1] // The commit SHA the note is attached to

		r, found, err := ReadRecord(repoRoot, objSha)
		if err != nil {
			return nil, err
		}
		if found {
			records = append(records, r)
		}
	}
	return records, nil
}

// Push pushes the notes ref to the remote.
func Push(repoRoot, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := git.Run(repoRoot, "push", remote, "refs/notes/"+NotesRef)
	return err
}

// Fetch fetches the notes ref from the remote.
func Fetch(repoRoot, remote string) error {
	if remote == "" {
		remote = "origin"
	}
	_, err := git.Run(repoRoot, "fetch", remote, fmt.Sprintf("refs/notes/%s:refs/notes/%s", NotesRef, NotesRef))
	return err
}
