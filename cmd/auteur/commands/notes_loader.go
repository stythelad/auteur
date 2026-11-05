package commands

import (
	"fmt"
	"sort"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/notes"
	"github.com/stythelad/auteur/internal/provenance"
)

// loadRecords loads provenance records from the shared notes ref.
// It verifies each signed envelope against the trust root.
// If the notes ref is empty or unavailable, it falls back to the local JSONL cache.
func loadRecords(root string, trustRoot map[string]string) ([]provenance.Record, error) {
	// Fetch shared notes into a separate ref and read them without importing
	// or re-signing them. The reader needs only a public trust root, never a
	// private SSH key. A configured remote fetch error is fatal: silently using
	// only local records would make CI's policy input incomplete.
	var remoteEnvelopes []notes.Envelope
	if _, remoteErr := git.Run(root, "remote", "get-url", "origin"); remoteErr == nil {
		fetched, fetchErr := notes.FetchEnvelopes(root, "origin")
		if fetchErr != nil {
			return nil, fmt.Errorf("fetch remote provenance notes: %w", fetchErr)
		}
		remoteEnvelopes = fetched
	}

	envelopes, err := notes.ListAllEnvelopes(root)
	if err != nil {
		return nil, fmt.Errorf("read local provenance notes: %w", err)
	}

	// Include the separately fetched remote envelopes. Do not write them back
	// under a new local signature: that would launder their provenance.
	seen := make(map[string]bool)
	for _, env := range envelopes {
		seen[env.Signature] = true
	}
	for _, env := range remoteEnvelopes {
		if !seen[env.Signature] {
			envelopes = append(envelopes, env)
			seen[env.Signature] = true
		}
	}
	if len(envelopes) == 0 {
		return provenance.Open(root).Load()
	}

	// Verify each envelope against the trust root.
	var recs []provenance.Record
	for _, env := range envelopes {
		if err := notes.VerifyEnvelope(&env, trustRoot); err != nil {
			return nil, fmt.Errorf("note verification failed for %s: %w", env.Identity, err)
		}
		recs = append(recs, env.Records...)
	}

	// Sort by timestamp ascending so LookupLine returns the newest match last.
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Timestamp.Before(recs[j].Timestamp)
	})

	return recs, nil
}
