package notes

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/stythelad/auteur/internal/provenance"
)

// Envelope wraps a set of records with a cryptographic signature.
type Envelope struct {
	Records   []provenance.Record `json:"records"`
	Identity  string              `json:"identity"`
	PublicKey string              `json:"public_key"`
	Signature string              `json:"signature"`
	Timestamp int64               `json:"timestamp"`
}

// SignEnvelope signs the records with the developer's SSH key.
func SignEnvelope(records []provenance.Record, identity, sshKeyPath string) (*Envelope, error) {
	b, err := json.Marshal(records)
	if err != nil {
		return nil, fmt.Errorf("marshal records: %w", err)
	}

	pubKey, sig, err := provenance.SignPayload(b, sshKeyPath)
	if err != nil {
		return nil, fmt.Errorf("sign payload: %w", err)
	}

	return &Envelope{
		Records:   records,
		Identity:  identity,
		PublicKey: pubKey,
		Signature: sig,
		Timestamp: time.Now().Unix(),
	}, nil
}

// VerifyEnvelope checks the envelope signature against the trusted signers map.
func VerifyEnvelope(env *Envelope, trustedSigners map[string]string) error {
	if len(trustedSigners) == 0 {
		return fmt.Errorf("cannot verify note: no trusted_signers configured")
	}

	trustedKey, ok := trustedSigners[env.Identity]
	if !ok {
		return fmt.Errorf("untrusted signer %q", env.Identity)
	}
	if trustedKey != env.PublicKey {
		return fmt.Errorf("public key mismatch for %q", env.Identity)
	}

	// Re-marshal the records to verify the signature
	payload, err := json.Marshal(env.Records)
	if err != nil {
		return err
	}

	if err := provenance.VerifySignature(string(payload), env.PublicKey, env.Signature); err != nil {
		return fmt.Errorf("invalid note signature: %w", err)
	}
	return nil
}
