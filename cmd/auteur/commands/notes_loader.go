package commands

import (
	"fmt"
	"os"
	"sort"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/notes"
	"github.com/stythelad/auteur/internal/provenance"
)

// loadRecords loads provenance records from the shared notes ref.
// It verifies each signed envelope against the trust root.
// If the notes ref is empty or unavailable, it falls back to the local JSONL cache.
func loadRecords(root string, trustRoot map[string]string) ([]provenance.Record, error) {
	// Try to fetch notes silently. If there's no remote, this will fail and we ignore it.
	if _, err := git.Run(root, "remote", "get-url", "origin"); err == nil {
		_ = notes.FetchAndMerge(root, "origin")
	}

	envelopes, err := notes.ListAllEnvelopes(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auteur: warning: could not read notes ref (%v); using local cache\n", err)
		return provenance.Open(root).Load()
	}

	if len(envelopes) == 0 {
		// Notes are empty or unsigned. Fall back to the local JSONL cache.
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
