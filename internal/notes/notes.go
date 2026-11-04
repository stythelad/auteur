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
// It signs the resulting envelope with the developer's key.
func WriteRecord(repoRoot, sha string, r provenance.Record) error {
	existing, found, err := ReadRecords(repoRoot, sha)
	if err != nil {
		return err
	}

	var records []provenance.Record
	if found {
		records = existing
	}
	records = append(records, r)

	// Determine the signer from git config
	identity, err := git.Run(repoRoot, "config", "user.name")
	if err != nil || identity == "" {
		return fmt.Errorf("cannot sign note: git user.name is not set")
	}

	env, err := SignEnvelope(records, identity, "")
	if err != nil {
		return fmt.Errorf("sign envelope: %w", err)
	}

	b, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
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

	// Try parsing as envelope first
	var env Envelope
	if err := json.Unmarshal([]byte(out), &env); err == nil && env.Signature != "" {
		return env.Records, true, nil
	}

	// Fallback to array (unsigned, old format)
	var records []provenance.Record
	if err := json.Unmarshal([]byte(out), &records); err == nil {
		return records, true, nil
	}

	// Fallback to single object
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

// ListAllEnvelopes returns all signed envelopes in the repo.
// This is used by loadRecords to verify signatures before trusting the data.
func ListAllEnvelopes(repoRoot string) ([]Envelope, error) {
	out, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "list")
	if err != nil {
		if strings.Contains(err.Error(), "no notes found") {
			return nil, nil
		}
		return nil, err
	}

	var envelopes []Envelope
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

		noteOut, err := git.Run(repoRoot, "notes", "--ref="+NotesRef, "show", objSha)
		if err != nil {
			continue
		}

		var env Envelope
		if err := json.Unmarshal([]byte(noteOut), &env); err == nil && env.Signature != "" {
			envelopes = append(envelopes, env)
		}
	}
	return envelopes, nil
}

// Push pushes the notes ref to the remote, handling non-fast-forward by merging and retrying.
func Push(repoRoot, remote string) error {
	if remote == "" {
		remote = "origin"
	}

	_, err := git.Run(repoRoot, "push", remote, "refs/notes/"+NotesRef)
	if err == nil {
		return nil
	}

	// If it failed, try to fetch+merge and retry with --force.
	if strings.Contains(err.Error(), "non-fast-forward") || strings.Contains(err.Error(), "rejected") {
		if mergeErr := FetchAndMerge(repoRoot, remote); mergeErr != nil {
			return fmt.Errorf("push failed and merge also failed: push=%v merge=%w", err, mergeErr)
		}
		_, retryErr := git.Run(repoRoot, "push", "--force", remote, "refs/notes/"+NotesRef)
		if retryErr != nil {
			return fmt.Errorf("push failed after merge: %w", retryErr)
		}
		return nil
	}

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
