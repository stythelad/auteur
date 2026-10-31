package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binaryPath is built once by TestMain.
var binaryPath string

// repo is a temp repo with auteur initialized.
type repo struct {
	t   *testing.T
	dir string
	env []string
}

func newRepo(t *testing.T) *repo {
	t.Helper()
	dir := t.TempDir()
	r := &repo{t: t, dir: dir}
	r.env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	r.git("init", "-q", "-b", "main")
	return r
}

// git runs a git command in the repo and fails the test on error.
func (r *repo) git(args ...string) string {
	r.t.Helper()
	out, err := r.gitErr(args...)
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

func (r *repo) gitErr(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// run runs the auteur binary in the repo and returns stdout+stderr.
func (r *repo) run(args ...string) (string, error) {
	r.t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = r.dir
	cmd.Env = r.env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// mustRun runs auteur and fails the test on error.
func (r *repo) mustRun(args ...string) string {
	r.t.Helper()
	out, err := r.run(args...)
	if err != nil {
		r.t.Fatalf("auteur %v: %v\n%s", args, err, out)
	}
	return out
}

// write writes a file relative to the repo root.
func (r *repo) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// appendTo appends content to a file.
func (r *repo) appendTo(path, content string) {
	r.t.Helper()
	f, err := os.OpenFile(filepath.Join(r.dir, path), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		r.t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		r.t.Fatal(err)
	}
}

// commitAll stages everything and commits with the given message.
func (r *repo) commitAll(msg string, extra ...string) string {
	r.t.Helper()
	r.git("add", "-A")
	args := append([]string{"commit", "-q", "-m", msg}, extra...)
	r.git(args...)
	return r.git("rev-parse", "HEAD")
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "auteur-bin-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	bin := filepath.Join(dir, "auteur")
	cmd := exec.Command("go", "build", "-o", bin, "../cmd/auteur")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("build failed: " + string(out))
	}
	binaryPath = bin
	os.Exit(m.Run())
}
