package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestHookAgentPreRejectsPathTraversal(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")

	// Attempt to trick the pre-hook into snapshotting /etc/passwd
	payload := `{"tool_input": {"file_path": "` + filepath.Join(r.dir, "..", "..", "etc", "passwd") + `"}}`
	cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected hook to exit 0 (silently ignore), got: %v\n%s", err, out)
	}

	// The output should contain the refusal message
	if !strings.Contains(string(out), "refusing to snapshot outside repo") {
		t.Fatalf("expected traversal refusal message, got: %s", out)
	}

	// The snapshot directory should not exist (or be empty)
	snapDir := filepath.Join(r.dir, ".git", "auteur", "snapshots")
	if entries, err := os.ReadDir(snapDir); err == nil && len(entries) > 0 {
		t.Fatalf("expected no snapshots to be created, found %d", len(entries))
	}
}

func TestSnapshotHashCollisions(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")

	// Create two files that would have collided under the old slash-to-underscore scheme
	r.write("foo/bar", "content A")
	r.write("foo_bar", "content B")

	// Simulate pre-hooks for both
	for _, file := range []string{"foo/bar", "foo_bar"} {
		fullPath := filepath.Join(r.dir, file)
		payload := `{"tool_input": {"file_path": "` + fullPath + `"}}`
		cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
		cmd.Dir = r.dir
		cmd.Env = r.env
		cmd.Stdin = strings.NewReader(payload)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("hook-agent-pre: %v\n%s", err, out)
		}
	}

	// Both snapshots should exist independently in the snapshots directory
	snapDir := filepath.Join(r.dir, ".git", "auteur", "snapshots")
	entries, err := os.ReadDir(snapDir)
	if err != nil {
		t.Fatalf("read snapshots dir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 snapshots (no collision), found %d", len(entries))
	}

	// Verify contents are distinct
	contentA, _ := os.ReadFile(filepath.Join(snapDir, entries[0].Name()))
	contentB, _ := os.ReadFile(filepath.Join(snapDir, entries[1].Name()))
	if string(contentA) == string(contentB) {
		t.Fatalf("snapshot collision! both files had content: %s", contentA)
	}
}

func TestSafeRelPath(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")

	// Helper to send the JSON payload to the pre-hook

	// Let's use a helper to send the JSON payload
	sendPayload := func(path string) (string, error) {
		payload := `{"tool_input": {"file_path": "` + path + `"}}`
		cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
		cmd.Dir = r.dir
		cmd.Env = r.env
		cmd.Stdin = strings.NewReader(payload)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	tests := []struct {
		name        string
		setup       func()
		path        string
		wantErr     bool
		expectSnaps int
	}{
		{
			name: "File symlink pointing to /etc/passwd",
			setup: func() {
				exec.Command("ln", "-s", "/etc/passwd", filepath.Join(r.dir, "badfile.txt")).Run()
			},
			path:        filepath.Join(r.dir, "badfile.txt"),
			wantErr:     true,
			expectSnaps: 0,
		},
		{
			name: "Directory symlink pointing outside",
			setup: func() {
				exec.Command("ln", "-s", "/tmp", filepath.Join(r.dir, "baddir")).Run()
			},
			path:        filepath.Join(r.dir, "baddir", "new.txt"),
			wantErr:     true,
			expectSnaps: 0,
		},
		{
			name: "New file beneath outside symlink",
			setup: func() {
				exec.Command("ln", "-s", "/tmp", filepath.Join(r.dir, "baddir2")).Run()
			},
			path:        filepath.Join(r.dir, "baddir2", "deep", "new.txt"),
			wantErr:     true,
			expectSnaps: 0,
		},
		{
			name: "Valid symlink inside repo",
			setup: func() {
				r.write("real.txt", "hello")
				exec.Command("ln", "-s", filepath.Join(r.dir, "real.txt"), filepath.Join(r.dir, "link.txt")).Run()
			},
			path:        filepath.Join(r.dir, "link.txt"),
			wantErr:     false,
			expectSnaps: 1,
		},
		{
			name:        "Ordinary new file",
			setup:       func() {},
			path:        filepath.Join(r.dir, "newfile.txt"),
			wantErr:     false,
			expectSnaps: 2,
		},
		{
			name:        "Existing file",
			setup:       func() {},
			path:        filepath.Join(r.dir, "real.txt"),
			wantErr:     false,
			expectSnaps: 2, // Same as valid symlink because it resolves to the same file
		},
	}

	snapDir := filepath.Join(r.dir, ".git", "auteur", "snapshots")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			out, err := sendPayload(tt.path)

			if tt.wantErr {
				if err == nil && !strings.Contains(out, "refusing") {
					// The hook swallows errors and returns exit 0, but prints "refusing"
					// so we check the output.
					t.Fatalf("expected refusal, got: %s", out)
				}
				if !strings.Contains(out, "refusing") {
					t.Fatalf("expected refusal message, got: %s", out)
				}
			} else {
				if strings.Contains(out, "refusing") {
					t.Fatalf("unexpected refusal: %s", out)
				}
			}

			// Check snapshot count
			entries, _ := os.ReadDir(snapDir)
			if len(entries) != tt.expectSnaps {
				t.Errorf("expected %d snapshots, got %d", tt.expectSnaps, len(entries))
			}
		})
	}
}

func TestHookAgentPreRejectsDanglingSymlink(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")

	// Create a dangling symlink inside the repo
	target := filepath.Join(r.dir, "dangling.txt")
	exec.Command("ln", "-s", "/tmp/does-not-exist-auteur-test", target).Run()

	// Send the payload to the hook
	payload := `{"tool_input": {"file_path": "` + target + `"}}`
	cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected hook to exit 0 (silently ignore), got: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "refusing to follow symlink") {
		t.Fatalf("expected symlink refusal, got: %s", out)
	}

	// Ensure no snapshot was created
	snapDir := filepath.Join(r.dir, ".git", "auteur", "snapshots")
	if entries, err := os.ReadDir(snapDir); err == nil && len(entries) > 0 {
		t.Fatalf("expected no snapshots, found %d", len(entries))
	}
}

func TestSecureReadFileRejectsSymlinkReplacement(t *testing.T) {
	// This simulates the TOCTOU race by directly calling the internal logic
	// via a small Go test harness. Since secureReadFile is not exported,
	// we can test it through the hook by creating a symlink.

	r := newRepo(t)
	r.mustRun("init")
	r.write("target.txt", "safe content")
	r.commitAll("base")

	// Create a symlink to /etc/passwd
	linkPath := filepath.Join(r.dir, "malicious.txt")
	exec.Command("ln", "-s", "/etc/passwd", linkPath).Run()

	payload := `{"tool_input": {"file_path": "` + linkPath + `"}}`
	cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected hook to exit 0, got: %v\n%s", err, out)
	}

	// The hook should refuse either at the path validation layer (outside repo)
	// or at the secure read layer (O_NOFOLLOW).
	if !strings.Contains(string(out), "refusing to follow symlink") && !strings.Contains(string(out), "outside the repository") {
		t.Fatalf("expected symlink refusal, got: %s", out)
	}
}

func TestHookAgentPreSpecialFiles(t *testing.T) {
	r := newRepo(t)
	r.mustRun("init")

	snapDir := filepath.Join(r.dir, ".git", "auteur", "snapshots")

	// Test 1: FIFO does not hang
	// Create a FIFO
	fifoPath := filepath.Join(r.dir, "evil_fifo")
	if err := exec.Command("mkfifo", fifoPath).Run(); err != nil {
		t.Skip("mkfifo not available on this platform")
	}

	payload := `{"tool_input": {"file_path": "` + fifoPath + `"}}`
	cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)

	// Set a timeout to ensure it doesn't hang
	done := make(chan bool)
	go func() {
		cmd.Run()
		done <- true
	}()

	select {
	case <-done:
		// Good, it didn't hang
	case <-time.After(3 * time.Second):
		t.Fatalf("SECURITY FAILURE: hook hung on FIFO")
	}

	// Verify no snapshot was created for the FIFO
	if entries, err := os.ReadDir(snapDir); err == nil {
		for _, e := range entries {
			content, _ := os.ReadFile(filepath.Join(snapDir, e.Name()))
			if strings.Contains(string(content), "fifo") {
				t.Fatalf("unexpected snapshot for FIFO: %s", e.Name())
			}
		}
	}

	// Test 2: Directory is rejected
	dirPath := filepath.Join(r.dir, "evil_dir")
	os.MkdirAll(dirPath, 0o755)
	payload = `{"tool_input": {"file_path": "` + dirPath + `"}}`
	cmd = exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "refusing to read non-regular file") {
		t.Fatalf("expected directory rejection, got: %s", out)
	}

	// Test 3: Normal file is read correctly
	normalPath := filepath.Join(r.dir, "normal.txt")
	r.write("normal.txt", "hello normal")
	payload = `{"tool_input": {"file_path": "` + normalPath + `"}}`
	cmd = exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)
	out, _ = cmd.CombinedOutput()
	if strings.Contains(string(out), "refusing") || strings.Contains(string(out), "error") {
		t.Fatalf("expected success for normal file, got: %s", out)
	}
	entries, _ := os.ReadDir(snapDir)
	if len(entries) == 0 {
		t.Fatalf("expected snapshot for normal file, found none")
	}

	// Test 4: Symlinks remain rejected
	outsideFile := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(outsideFile, []byte("secret"), 0o644)
	linkPath := filepath.Join(r.dir, "evil_link")
	os.Symlink(outsideFile, linkPath)

	payload = `{"tool_input": {"file_path": "` + linkPath + `"}}`
	cmd = exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = r.dir
	cmd.Env = r.env
	cmd.Stdin = strings.NewReader(payload)
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "outside the repository") && !strings.Contains(string(out), "refusing to follow symlink") {
		t.Fatalf("expected symlink rejection, got: %s", out)
	}
}
