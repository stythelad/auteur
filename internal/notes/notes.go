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

// FetchEnvelopes fetches the remote notes ref into an isolated local ref and
// returns its signed envelopes without importing or re-signing remote records.
// Reading shared provenance must never require the reader's private signing key.
func FetchEnvelopes(repoRoot, remote string) ([]Envelope, error) {
	if remote == "" {
		remote = "origin"
	}
	tmpRef := "refs/notes/auteur-remote"
	_, err := git.Run(repoRoot, "fetch", remote, "+"+ "refs/notes/"+NotesRef+":"+tmpRef)
	if err != nil {
		if strings.Contains(err.Error(), "couldn't find remote ref") || strings.Contains(err.Error(), "no such ref") {
			return nil, nil
		}
		return nil, fmt.Errorf("fetch remote notes: %w", err)
	}
	return ListEnvelopesFromRef(repoRoot, tmpRef)
}

// ListEnvelopesFromRef reads signed envelopes from a notes ref. Malformed or
// unsigned notes are errors rather than silently disappearing from policy input.
func ListEnvelopesFromRef(repoRoot, ref string) ([]Envelope, error) {
	refName := strings.TrimPrefix(ref, "refs/notes/")
	out, err := git.Run(repoRoot, "notes", "--ref="+refName, "list")
	if err != nil {
		if strings.Contains(err.Error(), "no notes found") {
			return nil, nil
		}
		return nil, fmt.Errorf("list notes %s: %w", ref, err)
	}
	var envelopes []Envelope
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) != 2 {
			if strings.TrimSpace(line) != "" {
				return nil, fmt.Errorf("malformed notes list entry %q", line)
			}
			continue
		}
		note, err := git.Run(repoRoot, "notes", "--ref="+refName, "show", parts[1])
		if err != nil {
			return nil, fmt.Errorf("read note for %s: %w", parts[1], err)
		}
		var env Envelope
		if err := json.Unmarshal([]byte(note), &env); err != nil {
			return nil, fmt.Errorf("parse signed note for %s: %w", parts[1], err)
		}
		if env.Signature == "" || len(env.Records) == 0 {
			return nil, fmt.Errorf("unsigned or empty remote note for %s", parts[1])
		}
		envelopes = append(envelopes, env)
	}
	return envelopes, nil
}

// ListAllEnvelopes returns all signed envelopes in the repo.
// This is used by loadRecords to verify signatures before trusting the data.
func ListAllEnvelopes(repoRoot string) ([]Envelope, error) {
	return ListEnvelopesFromRef(repoRoot, "refs/notes/"+NotesRef)
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

	// Never force-push notes after a race. A remote update may have landed
	// after the fetch, and --force would silently destroy that provenance.
	if strings.Contains(err.Error(), "non-fast-forward") || strings.Contains(err.Error(), "rejected") {
		if _, fetchErr := FetchEnvelopes(repoRoot, remote); fetchErr != nil {
			return fmt.Errorf("notes push rejected; fetching remote notes also failed: push=%v fetch=%w", err, fetchErr)
		}
		return fmt.Errorf("notes push rejected because the remote has newer provenance; remote notes were fetched to refs/notes/auteur-remote. Reconcile notes explicitly and retry (refusing to force-push): %w", err)
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
