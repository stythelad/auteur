package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTOCTOURaceParentDirectory(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binaryPath not set")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0o755)
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755)

	exec.Command("git", "-C", repoDir, "init", "-q").Run()
	exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", repoDir, "config", "user.name", "Alice").Run()

	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}

	// The real directory and file inside the repo
	realDir := filepath.Join(repoDir, "src")
	os.MkdirAll(realDir, 0o755)
	realFile := filepath.Join(realDir, "file.txt")
	os.WriteFile(realFile, []byte("safe"), 0o644)

	// The outside directory and file
	outsideDir := filepath.Join(tmpDir, "outside")
	os.MkdirAll(outsideDir, 0o755)
	outsideMarker := "OUTSIDE_SECRET_MARKER"
	outsideFile := filepath.Join(outsideDir, "file.txt")
	os.WriteFile(outsideFile, []byte(outsideMarker), 0o644)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Goroutine: repeatedly swap realDir with a symlink to outsideDir
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				os.RemoveAll(realDir)
				os.Symlink(outsideDir, realDir)

				os.Remove(realDir)
				os.MkdirAll(realDir, 0o755)
				os.WriteFile(realFile, []byte("safe"), 0o644)
			}
		}
	}()

	snapDir := filepath.Join(repoDir, ".git", "auteur", "snapshots")
	ledgerPath := filepath.Join(repoDir, ".git", "auteur", "provenance.jsonl")

	// Run hook many times and inspect snapshots each time
	for i := 0; i < 500; i++ {
		payload := `{"tool_input": {"file_path": "` + realFile + `"}}`
		cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
		cmd.Dir = repoDir
		cmd.Stdin = strings.NewReader(payload)
		out, _ := cmd.CombinedOutput()

		if strings.Contains(string(out), "panic") {
			t.Fatalf("hook panicked: %s", out)
		}

		// Inspect all snapshots for the outside marker
		entries, _ := os.ReadDir(snapDir)
		for _, e := range entries {
			content, _ := os.ReadFile(filepath.Join(snapDir, e.Name()))
			if strings.Contains(string(content), outsideMarker) {
				t.Fatalf("SECURITY FAILURE: snapshot contains outside marker!\nsnapshot: %s\ncontent: %s", e.Name(), content)
			}
		}

		// Verify no provenance records were created (since only pre-hook ran)
		if data, err := os.ReadFile(ledgerPath); err == nil && len(data) > 0 {
			t.Fatalf("expected no provenance records, found: %s", data)
		}
	}

	close(stop)
	wg.Wait()
}

func TestTOCTOURaceRejectedReadsCreateNoRecords(t *testing.T) {
	if binaryPath == "" {
		t.Skip("binaryPath not set")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0o755)
	exec.Command("git", "-C", repoDir, "init", "-q").Run()
	exec.Command("git", "-C", repoDir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", repoDir, "config", "user.name", "Alice").Run()

	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = repoDir
	cmd.Run()

	// Create an outside directory with a symlink inside the repo
	outsideDir := filepath.Join(tmpDir, "outside")
	os.MkdirAll(outsideDir, 0o755)
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	os.WriteFile(outsideFile, []byte("OUTSIDE_SECRET_MARKER"), 0o644)

	// Symlink inside the repo pointing to the outside file
	linkPath := filepath.Join(repoDir, "malicious.txt")
	os.Symlink(outsideFile, linkPath)

	// Run pre-hook (should be rejected)
	payload := `{"tool_input": {"file_path": "` + linkPath + `"}}`
	cmd = exec.Command(binaryPath, "hook-agent-pre", "claude-code")
	cmd.Dir = repoDir
	cmd.Stdin = strings.NewReader(payload)
	out, _ := cmd.CombinedOutput()

	if !strings.Contains(string(out), "outside the repository") && !strings.Contains(string(out), "refusing to follow symlink") {
		t.Fatalf("expected rejection, got: %s", out)
	}

	// Ledger should be empty (no records created)
	ledgerPath := filepath.Join(repoDir, ".git", "auteur", "provenance.jsonl")
	if data, err := os.ReadFile(ledgerPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		t.Fatalf("expected no provenance records, found: %s", data)
	}

	// Snapshots dir should not contain any real content
	snapDir := filepath.Join(repoDir, ".git", "auteur", "snapshots")
	if entries, err := os.ReadDir(snapDir); err == nil {
		for _, e := range entries {
			content, _ := os.ReadFile(filepath.Join(snapDir, e.Name()))
			if strings.Contains(string(content), "OUTSIDE_SECRET_MARKER") {
				t.Fatalf("snapshot leaked outside content: %s", content)
			}
		}
	}
}
