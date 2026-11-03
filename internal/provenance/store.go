package provenance

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

const LedgerDir = ".git/auteur"
const LedgerPath = ".git/auteur/provenance.jsonl"
const LockPath = ".git/auteur/lock"

type Store struct {
	mu   sync.Mutex
	path string
	lock string
}

// Open returns a Store rooted at the given repo root's .git directory.
func Open(repoRoot string) *Store {
	return &Store{
		path: filepath.Join(repoRoot, LedgerPath),
		lock: filepath.Join(repoRoot, LockPath),
	}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Init() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create auteur dir: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create ledger: %w", err)
	}
	return f.Close()
}

func (s *Store) lockFile() (func(), error) {
	f, err := os.OpenFile(s.lock, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// Basic unix flock
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}

func (s *Store) Append(r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Commit == "" {
		return errors.New("record missing commit")
	}

	unlock, err := s.lockFile()
	if err != nil {
		return fmt.Errorf("lock ledger: %w", err)
	}
	defer unlock()

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger for append: %w", err)
	}
	defer f.Close()

	recs, _ := s.Load()
	if len(recs) > 0 {
		r.PrevHash = recs[len(recs)-1].Hash
	}
	r.Hash = computeHash(r)

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("encode record: %w", err)
	}
	return nil
}

func (s *Store) Load() ([]Record, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ledger: %w", err)
	}
	defer f.Close()

	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("ledger line %d: %w", line, err)
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("scan ledger: %w", err)
	}
	return out, nil
}

func (s *Store) FindByCommit(sha string) (Record, bool, error) {
	recs, err := s.Load()
	if err != nil {
		return Record{}, false, err
	}
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].Commit == sha {
			return recs[i], true, nil
		}
	}
	return Record{}, false, nil
}

func (s *Store) Rewrite(recs []Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	unlock, err := s.lockFile()
	if err != nil {
		return fmt.Errorf("lock ledger: %w", err)
	}
	defer unlock()

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger for rewrite: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	var prevHash string
	for i := range recs {
		recs[i].PrevHash = prevHash
		recs[i].Hash = computeHash(recs[i])
		if err := enc.Encode(recs[i]); err != nil {
			return fmt.Errorf("encode record: %w", err)
		}
		prevHash = recs[i].Hash
	}
	return nil
}
