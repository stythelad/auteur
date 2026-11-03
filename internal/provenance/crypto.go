package provenance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func computeHash(r Record) string {
	r.Hash = ""
	saved := make([]Signoff, len(r.Review.Signoffs))
	for i := range r.Review.Signoffs {
		saved[i] = r.Review.Signoffs[i]
		saved[i].Signature = ""
	}
	r.Review.Signoffs = saved
	b, _ := json.Marshal(r)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func VerifyLedger(recs []Record) error {
	var prevHash string
	for i, r := range recs {
		if i > 0 && r.PrevHash != prevHash {
			return fmt.Errorf("tampering detected at record %d: prev_hash mismatch", i)
		}
		if r.Hash != "" && r.Hash != computeHash(r) {
			return fmt.Errorf("tampering detected at record %d: hash mismatch", i)
		}
		prevHash = r.Hash
	}
	return nil
}

func GetPublicKey(sshKeyPath string) (string, error) {
	if sshKeyPath == "" {
		home, _ := os.UserHomeDir()
		sshKeyPath = filepath.Join(home, ".ssh", "id_ed25519")
	}
	pubKeyPath := sshKeyPath + ".pub"
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return "", fmt.Errorf("read public key: %w", err)
	}
	pubParts := strings.Fields(string(pubKeyBytes))
	if len(pubParts) < 2 {
		return "", fmt.Errorf("invalid public key format")
	}
	return pubParts[0] + " " + pubParts[1], nil
}

func SignHashBytes(hash string, sshKeyPath string) (string, error) {
	if sshKeyPath == "" {
		home, _ := os.UserHomeDir()
		sshKeyPath = filepath.Join(home, ".ssh", "id_ed25519")
	}

	tmpData := filepath.Join(".", "auteur-tmp-sig-data")
	if err := os.WriteFile(tmpData, []byte(hash), 0644); err != nil {
		return "", fmt.Errorf("write temp data: %w", err)
	}
	defer os.Remove(tmpData)
	defer os.Remove(tmpData + ".sig")

	cmd := exec.Command("ssh-keygen", "-Y", "sign", "-n", "auteur", "-f", sshKeyPath, tmpData)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ssh-keygen sign failed: %w: %s", err, out)
	}

	sigBytes, err := os.ReadFile(tmpData + ".sig")
	if err != nil {
		return "", fmt.Errorf("read signature file: %w", err)
	}
	if len(sigBytes) == 0 {
		return "", fmt.Errorf("ssh-keygen produced an empty signature")
	}
	return string(sigBytes), nil
}

func VerifySignature(hash string, pubKey string, sig string) error {
	tmpDir, _ := os.MkdirTemp("", "auteur-verify-*")
	defer os.RemoveAll(tmpDir)

	tmpSig := filepath.Join(tmpDir, "data.sig")
	allowedSigners := filepath.Join(tmpDir, "allowed")

	os.WriteFile(tmpSig, []byte(sig), 0644)
	os.WriteFile(allowedSigners, []byte("signer "+pubKey+"\n"), 0644)

	cmd := exec.Command("ssh-keygen", "-Y", "verify", "-f", allowedSigners, "-I", "signer", "-n", "auteur", "-s", tmpSig)
	cmd.Stdin = bytes.NewReader([]byte(hash))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("signature verification failed: %w: %s", err, out)
	}
	return nil
}

func SignRecord(r *Record, identity string, sshKeyPath string) error {
	r.Review.Signoffs = append(r.Review.Signoffs, Signoff{
		Identity:  identity,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})

	pubKey, err := GetPublicKey(sshKeyPath)
	if err != nil {
		return err
	}
	r.Review.Signoffs[len(r.Review.Signoffs)-1].PublicKey = pubKey

	hash := computeHash(*r)

	sig, err := SignHashBytes(hash, sshKeyPath)
	if err != nil {
		return err
	}

	r.Review.Signoffs[len(r.Review.Signoffs)-1].Signature = sig
	r.Hash = computeHash(*r)
	return nil
}

func VerifySignoffs(recs []Record, trustedSigners map[string]string) error {
	for _, r := range recs {
		if len(r.Review.Signoffs) == 0 {
			continue
		}

		saved := make([]string, len(r.Review.Signoffs))
		for i := range r.Review.Signoffs {
			saved[i] = r.Review.Signoffs[i].Signature
			r.Review.Signoffs[i].Signature = ""
		}
		hashToVerify := computeHash(r)
		for i := range r.Review.Signoffs {
			r.Review.Signoffs[i].Signature = saved[i]
		}

		for _, s := range r.Review.Signoffs {
			// Check against trusted keys if provided
			if len(trustedSigners) > 0 {
				trustedKey, ok := trustedSigners[s.Identity]
				if !ok {
					return fmt.Errorf("untrusted signer %q for commit %s (key not in config)", s.Identity, r.Commit[:7])
				}
				if trustedKey != s.PublicKey {
					return fmt.Errorf("public key mismatch for %q on commit %s: the key used to sign does not match the trusted key in config", s.Identity, r.Commit[:7])
				}
			}

			if err := VerifySignature(hashToVerify, s.PublicKey, s.Signature); err != nil {
				return fmt.Errorf("invalid signature for commit %s by %s: %w", r.Commit[:7], s.Identity, err)
			}
		}
	}
	return nil
}
