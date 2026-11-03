package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// computeHash generates a stable SHA-256 hash of a record, ignoring the Hash field itself.
func computeHash(r Record) string {
	r.Hash = ""
	b, _ := json.Marshal(r)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// VerifyLedger checks if the hash chain is intact.
func VerifyLedger(recs []Record) error {
	var prevHash string
	for i, r := range recs {
		if i > 0 && r.PrevHash != prevHash {
			return fmt.Errorf("ledger tampering detected at record %d (commit %s): prev_hash mismatch", i, r.Commit)
		}
		expected := computeHash(r)
		if r.Hash != "" && r.Hash != expected {
			return fmt.Errorf("ledger tampering detected at record %d (commit %s): hash mismatch", i, r.Commit)
		}
		prevHash = r.Hash
	}
	return nil
}
