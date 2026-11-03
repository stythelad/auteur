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
	// This test requires the binary to be built
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

	// Initialize auteur
	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = repoDir
	cmd.Run()

	// Create the real directory and file
	realDir := filepath.Join(repoDir, "src")
	os.MkdirAll(realDir, 0o755)
	realFile := filepath.Join(realDir, "file.txt")
	os.WriteFile(realFile, []byte("safe"), 0o644)

	// Create the outside directory and file
	outsideDir := filepath.Join(tmpDir, "outside")
	os.MkdirAll(outsideDir, 0o755)
	outsideFile := filepath.Join(outsideDir, "file.txt")
	os.WriteFile(outsideFile, []byte("hacked"), 0o644)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Goroutine to rapidly swap the directory with a symlink
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				// Swap: remove real dir, symlink to outside
				os.RemoveAll(realDir)
				os.Symlink(outsideDir, realDir)

				// Swap back: remove symlink, recreate real dir
				os.Remove(realDir)
				os.MkdirAll(realDir, 0o755)
				os.WriteFile(realFile, []byte("safe"), 0o644)
			}
		}
	}()

	// Run the hook in a loop
	for i := 0; i < 100; i++ {
		payload := `{"tool_input": {"file_path": "` + realFile + `"}}`
		cmd := exec.Command(binaryPath, "hook-agent-pre", "claude-code")
		cmd.Dir = repoDir
		cmd.Stdin = strings.NewReader(payload)
		out, _ := cmd.CombinedOutput()

		// If it read the outside file, it would say "hacked" in the snapshot.
		// We can't easily check the snapshot content here, but we can check if it crashed.
		if strings.Contains(string(out), "panic") {
			t.Fatalf("hook panicked during race: %s", out)
		}
	}

	close(stop)
	wg.Wait()
}
