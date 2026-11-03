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
	// When verifying signoffs, we don't want to include the signoffs themselves
	// in the hash, otherwise adding a signature invalidates previous ones.
	r.Review.Signoffs = nil
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

func SignHash(hash string, sshKeyPath string) (pubKey, signature string, err error) {
	if sshKeyPath == "" {
		home, _ := os.UserHomeDir()
		sshKeyPath = filepath.Join(home, ".ssh", "id_ed25519")
	}
	pubKeyPath := sshKeyPath + ".pub"
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return "", "", fmt.Errorf("read public key: %w", err)
	}

	tmp, _ := os.CreateTemp("", "auteur-sig-*")
	tmp.WriteString(hash)
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.Command("ssh-keygen", "-Y", "sign", "-n", "auteur", "-f", sshKeyPath, tmp.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("ssh-keygen sign: %w: %s", err, out)
	}

	sigBytes, err := os.ReadFile(tmp.Name() + ".sig")
	if err != nil {
		return "", "", fmt.Errorf("read signature: %w", err)
	}

	return strings.TrimSpace(string(pubKeyBytes)), string(sigBytes), nil
}

func VerifySignature(hash string, pubKey string, sig string) error {
	tmpDir, _ := os.MkdirTemp("", "auteur-verify-*")
	defer os.RemoveAll(tmpDir)

	tmpData := filepath.Join(tmpDir, "data")
	tmpSig := filepath.Join(tmpDir, "data.sig")
	tmpPub := filepath.Join(tmpDir, "key.pub")
	allowedSigners := filepath.Join(tmpDir, "allowed")

	os.WriteFile(tmpData, []byte(hash), 0644)
	os.WriteFile(tmpSig, []byte(sig), 0644)
	os.WriteFile(tmpPub, []byte(pubKey), 0644)

	// The format is "identity" followed by the public key
	os.WriteFile(allowedSigners, []byte("signer "+pubKey), 0644)

	cmd := exec.Command("ssh-keygen", "-Y", "verify", "-f", allowedSigners, "-I", "signer", "-n", "auteur", "-s", tmpSig)
	cmd.Stdin = bytes.NewReader([]byte(hash))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("signature verification failed: %w: %s", err, out)
	}
	return nil
}

func SignRecord(r *Record, identity string, sshKeyPath string) error {
	// 1. Compute the hash of the record AS IT IS NOW (without the new signature)
	hash := computeHash(*r)

	// 2. Sign that hash
	pubKey, sig, err := SignHash(hash, sshKeyPath)
	if err != nil {
		return err
	}

	// 3. Append the signoff
	r.Review.Signoffs = append(r.Review.Signoffs, Signoff{
		Identity:  identity,
		PublicKey: pubKey,
		Signature: sig,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})

	// 4. Recompute the final record hash
	r.Hash = computeHash(*r)
	return nil
}

func VerifySignoffs(recs []Record) error {
	for _, r := range recs {
		for _, s := range r.Review.Signoffs {
			// To verify, we must recompute the hash of the record WITHOUT its signatures.
			// We make a copy, remove the signoffs, and compute.
			temp := r
			temp.Review.Signoffs = nil
			hashToVerify := computeHash(temp)

			if err := VerifySignature(hashToVerify, s.PublicKey, s.Signature); err != nil {
				return fmt.Errorf("invalid signature for commit %s by %s: %w", r.Commit[:7], s.Identity, err)
			}
		}
	}
	return nil
}
