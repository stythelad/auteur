package provenance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func generateTestKey(t *testing.T, dir, name string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", path, "-N", "", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, out)
	}
	pubBytes, err := os.ReadFile(path + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	return path, string(pubBytes)
}

func TestVerifySignoffs_Security(t *testing.T) {
	tmpDir := t.TempDir()
	aliceKeyPath, alicePub := generateTestKey(t, tmpDir, "alice")
	attackerKeyPath, attackerPub := generateTestKey(t, tmpDir, "attacker")

	makeRecord := func(id string, keyPath string) Record {
		r := Record{
			Commit:     "abc1234567890",
			Timestamp:  time.Now().UTC(),
			Confidence: ConfidenceHigh,
			Lines:      []LineRange{{File: "auth/login.py", Start: 1, End: 1, Origin: OriginAgent}},
		}
		if err := SignRecord(&r, id, keyPath); err != nil {
			t.Fatalf("SignRecord: %v", err)
		}
		return r
	}

	parsePub := func(raw string) string {
		parts := strings.Fields(raw)
		return parts[0] + " " + parts[1]
	}

	alicePubClean := parsePub(alicePub)
	attackerPubClean := parsePub(attackerPub)
	_ = attackerPubClean

	tests := []struct {
		name           string
		record         Record
		trustedSigners map[string]string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "T1: Default config, forged signoff as 'Alice' (no trust root)",
			record:         makeRecord("Alice", attackerKeyPath),
			trustedSigners: nil,
			wantErr:        true,
			errContains:    "no trusted_signers are configured",
		},
		{
			name:           "T2: Trust list set, attacker signs as name not in list",
			record:         makeRecord("Mallory", attackerKeyPath),
			trustedSigners: map[string]string{"Alice": alicePubClean},
			wantErr:        true,
			errContains:    "untrusted signer",
		},
		{
			name:           "T3: Trust list set, attacker claims 'Alice' with their own key",
			record:         makeRecord("Alice", attackerKeyPath),
			trustedSigners: map[string]string{"Alice": alicePubClean},
			wantErr:        true,
			errContains:    "public key mismatch",
		},
		{
			name:           "T4: Trust list set, real Alice signs",
			record:         makeRecord("Alice", aliceKeyPath),
			trustedSigners: map[string]string{"Alice": alicePubClean},
			wantErr:        false,
		},
		{
			name: "T5: Tampered identity in ledger",
			record: func() Record {
				r := makeRecord("Alice", aliceKeyPath)
				r.Review.Signoffs[0].Identity = "Hacker"
				return r
			}(),
			trustedSigners: map[string]string{"Alice": alicePubClean},
			wantErr:        true,
			errContains:    "untrusted signer",
		},
		{
			name: "T6: Tampered signature in ledger",
			record: func() Record {
				r := makeRecord("Alice", aliceKeyPath)
				r.Review.Signoffs[0].Signature = "bad signature"
				return r
			}(),
			trustedSigners: map[string]string{"Alice": alicePubClean},
			wantErr:        true,
			errContains:    "signature verification failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs := []Record{tt.record}
			err := VerifySignoffs(recs, tt.trustedSigners)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
			}
		})
	}
}
