package commands

import (
	"fmt"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/notes"
	"github.com/stythelad/auteur/internal/provenance"
)

// loadRecords loads provenance records from the shared notes ref.
// If the notes ref is empty or unavailable (e.g., local development with no remote),
// it falls back to the local JSONL cache.
func loadRecords(root string) ([]provenance.Record, error) {
	// Try to fetch notes silently. If there's no remote, this will fail and we ignore it.
	if _, err := git.Run(root, "remote", "get-url", "origin"); err == nil {
		_ = notes.Fetch(root, "origin")
	}

	recs, err := notes.ListAllRecords(root)
	if err != nil {
		// If listing notes fails, fall back to the JSONL cache.
		fmt.Printf("auteur: warning: could not read notes ref (%v); using local cache\n", err)
		return provenance.Open(root).Load()
	}

	if len(recs) == 0 {
		// Notes are empty. This is normal for a fresh clone or local dev.
		// Fall back to the local JSONL cache.
		return provenance.Open(root).Load()
	}

	return recs, nil
}
