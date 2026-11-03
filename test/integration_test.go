package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHappyPath(t *testing.T) {
	r := newRepo(t)
	r.write("auth/login.py", "func login() {}\n")
	r.commitAll("initial")

	r.mustRun("init")

	out := r.mustRun("wrap", "--agent", "claude-code", "--model", "claude-opus-5.5", "--",
		"sh", "-c", "echo 'func evil() {}' >> auth/login.py")
	if !strings.Contains(out, "recorded pending") {
		t.Fatalf("wrap output missing recorded: %s", out)
	}

	sha := r.commitAll("agent changes")

	out = r.mustRun("pending", "list")
	if !strings.Contains(out, "no pending records") {
		t.Fatalf("pending not resolved: %s", out)
	}

	out = r.mustRun("blame", "auth/login.py:2")
	if !strings.Contains(out, sha[:7]) {
		t.Fatalf("blame missing real sha %s: %s", sha[:7], out)
	}
	if !strings.Contains(out, "confidence: high") {
		t.Fatalf("blame missing high confidence: %s", out)
	}
}

func TestScanHistory(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "one\n")
	r.commitAll("initial")

	r.write("a.txt", "one\ntwo\n")
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "add two", "-m", "Co-Authored-By: Claude <noreply@anthropic.com>")

	r.mustRun("init")
	out := r.mustRun("scan-history")
	if !strings.Contains(out, "recorded 1") {
		t.Fatalf("expected 1 recorded, got: %s", out)
	}

	out = r.mustRun("blame", "a.txt:2")
	if !strings.Contains(out, "confidence: medium") {
		t.Fatalf("expected medium confidence: %s", out)
	}
	if !strings.Contains(out, "claude-code") {
		t.Fatalf("expected claude-code: %s", out)
	}
}

func TestCheckFailsOnAgentInProtectedPath(t *testing.T) {
	r := newRepo(t)
	r.write("auth/login.py", "func login() {}\n")
	r.commitAll("initial")
	r.mustRun("init")

	r.mustRun("wrap", "--agent", "claude-code", "--",
		"sh", "-c", "echo 'func evil() {}' >> auth/login.py")
	r.commitAll("agent change")

	out, err := r.run("check")
	if err == nil {
		t.Fatalf("expected check to fail, got: %s", out)
	}
	if !strings.Contains(out, "VIOLATION") {
		t.Fatalf("expected VIOLATION in output: %s", out)
	}
	if !strings.Contains(out, "protected-paths-need-signoff") {
		t.Fatalf("expected rule name in output: %s", out)
	}
}

func TestCheckPassesOnCleanRepo(t *testing.T) {
	r := newRepo(t)
	r.write("README.md", "# hi\n")
	r.commitAll("initial")
	r.mustRun("init")

	out, err := r.run("check")
	if err != nil {
		t.Fatalf("expected pass on clean repo, got: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no violations") {
		t.Fatalf("expected no violations: %s", out)
	}
}

func TestCheckFailsOnMissingLedger(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "x\n")
	r.commitAll("initial")

	out, err := r.run("check")
	if err == nil {
		t.Fatalf("expected check to fail on missing ledger, got: %s", out)
	}
	if !strings.Contains(out, "ledger not found") {
		t.Fatalf("expected ledger-not-found error: %s", out)
	}

	if _, err := r.run("check", "--allow-missing-ledger"); err != nil {
		t.Fatalf("allow-missing-ledger should pass: %v", err)
	}
}

func TestCheckFailsOnMalformedPolicy(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "x\n")
	r.commitAll("initial")
	r.mustRun("init")

	policyPath := filepath.Join(r.dir, ".auteur", "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("version: 1\nrules:\n  - name: r\n    paths: [\"a/**\"]\n    require_human_singoff: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := r.run("check")
	if err == nil {
		t.Fatalf("expected check to fail on bad policy, got: %s", out)
	}
}

func TestWrapNoChanges(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "x\n")
	r.commitAll("initial")
	r.mustRun("init")

	out := r.mustRun("wrap", "--agent", "claude-code", "--", "true")
	if !strings.Contains(out, "no changes detected") {
		t.Fatalf("expected no-changes message: %s", out)
	}

	out = r.mustRun("pending", "list")
	if !strings.Contains(out, "no pending records") {
		t.Fatalf("expected no pending: %s", out)
	}
}

func TestAmendPreservesAttribution(t *testing.T) {
	r := newRepo(t)
	r.write("auth/login.py", "func login() {}\n")
	r.commitAll("initial")
	r.mustRun("init")

	r.mustRun("wrap", "--agent", "claude-code", "--",
		"sh", "-c", "echo 'func evil() {}' >> auth/login.py")
	r.commitAll("agent changes")

	// Amend the commit; post-rewrite should copy the record to the new SHA.
	r.git("commit", "--amend", "-q", "--no-edit")
	newSHA := r.git("rev-parse", "HEAD")

	out := r.mustRun("blame", "auth/login.py:2")
	if !strings.Contains(out, newSHA[:7]) {
		t.Fatalf("blame should show amended SHA %s: %s", newSHA[:7], out)
	}
	if !strings.Contains(out, "confidence: high") {
		t.Fatalf("confidence lost after amend: %s", out)
	}
}

func TestRebasePreservesAttribution(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "base\n")
	r.commitAll("base")

	r.git("checkout", "-q", "-b", "feature")
	r.mustRun("init")
	r.mustRun("wrap", "--agent", "claude-code", "--",
		"sh", "-c", "echo 'feature' >> a.txt")
	r.commitAll("agent feature")

	r.git("checkout", "-q", "main")
	r.write("b.txt", "main\n")
	r.commitAll("main advance")

	r.git("checkout", "-q", "feature")
	r.git("rebase", "-q", "main")
	newSHA := r.git("rev-parse", "HEAD")

	out := r.mustRun("blame", "a.txt:2")
	if !strings.Contains(out, newSHA[:7]) {
		t.Fatalf("blame should show rebased SHA %s after rebase: %s", newSHA[:7], out)
	}
	if !strings.Contains(out, "confidence: high") {
		t.Fatalf("confidence lost after rebase: %s", out)
	}
}

func TestSignFailsIfTrustRootMissing(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "x\n")
	r.commitAll("base")
	r.mustRun("init")
	keyPath := generateTestKey(t, r.dir)

	r.mustRun("wrap", "--agent", "claude-code", "--",
		"sh", "-c", "echo 'y' >> a.txt")
	r.commitAll("agent edit")

	// Do NOT create .auteur/allowed_signers
	out, err := r.run("sign", "--ssh-key", keyPath)
	if err == nil {
		t.Fatalf("expected sign to fail without trust root, got: %s", out)
	}
	if !strings.Contains(out, "no trusted signers configured") {
		t.Fatalf("expected missing trust root error, got: %s", out)
	}
}

func TestSignFailsIfTrustRootEmpty(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "x\n")
	r.commitAll("base")
	r.mustRun("init")
	keyPath := generateTestKey(t, r.dir)

	r.mustRun("wrap", "--agent", "claude-code", "--",
		"sh", "-c", "echo 'y' >> a.txt")
	r.commitAll("agent edit")

	// Create an empty trust root
	r.write(".auteur/allowed_signers", "trusted_signers: {}\n")

	out, err := r.run("sign", "--ssh-key", keyPath)
	if err == nil {
		t.Fatalf("expected sign to fail with empty trust root, got: %s", out)
	}
	if !strings.Contains(out, "no trusted signers configured") {
		t.Fatalf("expected missing trust root error, got: %s", out)
	}
}

func TestSignFailsIfKeyUntrusted(t *testing.T) {
	r := newRepo(t)
	r.write("a.txt", "x\n")
	r.commitAll("base")
	r.mustRun("init")
	keyPath := generateTestKey(t, r.dir)

	r.mustRun("wrap", "--agent", "claude-code", "--",
		"sh", "-c", "echo 'y' >> a.txt")
	r.commitAll("agent edit")

	// Add a *fake* key to the trust root
	r.write(".auteur/allowed_signers", "trusted_signers:\n  \"test\": \"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFakeKey\"\n")

	out, err := r.run("sign", "--ssh-key", keyPath)
	if err == nil {
		t.Fatalf("expected sign to fail with untrusted key, got: %s", out)
	}
	if !strings.Contains(out, "your public key is not in the trusted signers list") {
		t.Fatalf("expected untrusted key error, got: %s", out)
	}
}

func TestCheckLineLevelProvenance(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")
	r.write("auth/login.py", "line 1\n")
	// Force-add the policy and trust root to main
	r.write(".auteur/allowed_signers", "trusted_signers:\n  \"test\": \"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI\"\n")
	r.git("add", "-f", ".auteur/policy.yaml")
	r.git("add", "-f", ".auteur/allowed_signers")
	r.commitAll("base and policy")

	r.git("checkout", "-q", "-b", "feature")

	// AI edit: adds line 2
	r.mustRun("wrap", "--agent", "claude-code", "--",
		"sh", "-c", "echo 'line 2' >> auth/login.py")
	r.commitAll("ai edit")

	// Human edit: adds line 3 (no provenance)
	r.appendTo("auth/login.py", "line 3\n")
	r.commitAll("human edit")

	// Check against main
	out, err := r.run("check", "--ci-base", "main")
	if err == nil {
		t.Fatalf("expected check to fail, got: %s", out)
	}

	// The AI's line 2 should be flagged (needs signoff)
	if !strings.Contains(out, "auth/login.py:2-2") {
		t.Fatalf("expected violation on line 2, got: %s", out)
	}
	// The human's line 3 should be flagged (no record)
	if !strings.Contains(out, "auth/login.py:3-3") {
		t.Fatalf("expected violation on line 3, got: %s", out)
	}
}

func TestCIRejectsMissingBaseTrustRoot(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")
	r.write("auth/login.py", "base\n")
	r.commitAll("base")

	r.git("checkout", "-q", "-b", "feature")
	r.write("auth/login.py", "base\nevil\n")
	r.commitAll("attack")

	out, err := r.run("check", "--ci-base", "main")
	if err == nil {
		t.Fatalf("expected error, got: %s", out)
	}
	if !strings.Contains(out, "does not contain .auteur/allowed_signers") {
		t.Fatalf("expected missing trust root error, got: %s", out)
	}
}

func TestCIRejectsPRTrustRootModification(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")
	r.write("auth/login.py", "base\n")
	// Write the trust root manually (SSH keys in CI tests are tricky)
	r.write(".auteur/allowed_signers", "trusted_signers:\n  \"test\": \"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI\"\n")
	r.git("add", "-f", ".auteur/allowed_signers")
	r.commitAll("base + trust root")

	r.git("checkout", "-q", "-b", "feature")
	r.write(".auteur/allowed_signers", "trusted_signers:\n  \"test\": \"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI\"\n  \"hacker\": \"ssh-ed25519 HACKER\"\n")
	r.git("add", "-f", ".auteur/allowed_signers")
	r.commitAll("attacker modifies trust root")

	out, err := r.run("check", "--ci-base", "main")
	if err == nil {
		t.Fatalf("expected error, got: %s", out)
	}
	if !strings.Contains(out, "PR modifies the trust root") {
		t.Fatalf("expected trust root modification error, got: %s", out)
	}
}

func TestPromptRedaction(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")

	// Enable prompt capture for this test
	cfgPath := filepath.Join(r.dir, ".git", "auteur", "config.yaml")
	cfgData, _ := os.ReadFile(cfgPath)
	os.WriteFile(cfgPath, []byte(strings.Replace(string(cfgData), "capture_prompt: false", "capture_prompt: true", 1)), 0o644)

	r.write("auth/login.py", "base\n")
	r.commitAll("base")

	// Simulate an AI edit with a secret in the prompt
	payload := `{"prompt": "use AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "tool_input": {"file_path": "` + filepath.Join(r.dir, "auth", "login.py") + `"}}`
	cmd := exec.Command(binaryPath, "hook-agent", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("hook-agent: %v\n%s", err, out)
	}

	r.appendTo("auth/login.py", "evil\n")
	r.commitAll("ai edit")

	ledger, _ := os.ReadFile(filepath.Join(r.dir, ".git", "auteur", "provenance.jsonl"))
	if strings.Contains(string(ledger), "wJalrXUtnFEMI") {
		t.Fatalf("secret was not redacted in ledger: %s", ledger)
	}
	if !strings.Contains(string(ledger), "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in ledger: %s", ledger)
	}
}
func generateTestKey(t *testing.T, dir string) string {
	t.Helper()
	keyPath := filepath.Join(dir, "test_key")
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v: %s", err, out)
	}
	return keyPath
}
