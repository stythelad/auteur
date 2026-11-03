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
)

// LedgerPath is the canonical location of the provenance file,
// relative to the repo root.
const LedgerPath = ".auteur/provenance.jsonl"

// Store is an append-only JSONL ledger.
// It is safe for concurrent use within a single process.
type Store struct {
	mu   sync.Mutex
	path string
}

// Open returns a Store rooted at the given repo root.
// It does not create the file; call Init if the ledger may not exist yet.
func Open(repoRoot string) *Store {
	return &Store{path: filepath.Join(repoRoot, LedgerPath)}
}

// Path returns the absolute path of the ledger file.
func (s *Store) Path() string { return s.path }

// Init creates the .auteur directory and an empty ledger if missing.
// It is idempotent.
func (s *Store) Init() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create .auteur dir: %w", err)
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create ledger: %w", err)
	}
	return f.Close()
}

// Append writes a single record to the ledger.
// Records are newline-delimited JSON objects.
func (s *Store) Append(r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Commit == "" {
		return errors.New("record missing commit")
	}

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger for append: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("encode record: %w", err)
	}
	return nil
}

// Load reads every record from the ledger in file order.
// Malformed lines are returned as an error listing the line number,
// so callers can decide whether to skip or fail hard.
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
	// Bump the scanner buffer: a single record can be large for big commits.
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

// FindByCommit returns the most recent record for the given commit SHA,
// or (zero, false) if none.
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

// Rewrite overwrites the entire ledger file with the provided records.
// This is used for mutations like sign-offs, where appending isn't sufficient.
func (s *Store) Rewrite(recs []Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger for rewrite: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			return fmt.Errorf("encode record: %w", err)
		}
	}
	return nil
}
