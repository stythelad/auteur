package test

import (
	"os"
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
