package provenance

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTempStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s := Open(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("init: %v", err)
	}
	return s, dir
}

func sampleRecord() Record {
	return Record{
		Commit:     "a1b2c3d",
		Timestamp:  time.Date(2026, 10, 31, 14, 0, 0, 0, time.UTC),
		Author:     Author{Name: "alice", Email: "alice@example.com"},
		Agent:      &Agent{Name: "claude-code", Model: "claude-opus-5.5"},
		Confidence: ConfidenceHigh,
		Lines: []LineRange{
			{File: "a.go", Start: 1, End: 10, Origin: OriginAgent},
			{File: "a.go", Start: 11, End: 15, Origin: OriginHuman},
		},
	}
}

func TestInitAndAppendLoad(t *testing.T) {
	s, dir := newTempStore(t)
	if _, err := os.Stat(filepath.Join(dir, LedgerPath)); err != nil {
		t.Fatalf("ledger not created: %v", err)
	}

	if err := s.Append(sampleRecord()); err != nil {
		t.Fatalf("append: %v", err)
	}
	recs, err := s.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("want 1 record, got %d", len(recs))
	}
	if recs[0].Commit != "a1b2c3d" {
		t.Errorf("commit mismatch: %q", recs[0].Commit)
	}
	if len(recs[0].Lines) != 2 {
		t.Errorf("want 2 line ranges, got %d", len(recs[0].Lines))
	}
}

func TestAppendRequiresCommit(t *testing.T) {
	s, _ := newTempStore(t)
	r := sampleRecord()
	r.Commit = ""
	if err := s.Append(r); err == nil {
		t.Fatal("expected error for missing commit")
	}
}

func TestLoadMissingFile(t *testing.T) {
	dir := t.TempDir()
	s := Open(dir) // not Init'd
	recs, err := s.Load()
	if err != nil {
		t.Fatalf("load on missing file: %v", err)
	}
	if recs != nil {
		t.Errorf("want nil, got %v", recs)
	}
}

func TestFindByCommit(t *testing.T) {
	s, _ := newTempStore(t)
	r1 := sampleRecord()
	r1.Commit = "aaa"
	r2 := sampleRecord()
	r2.Commit = "bbb"
	_ = s.Append(r1)
	_ = s.Append(r2)

	got, ok, err := s.FindByCommit("bbb")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.Commit != "bbb" {
		t.Fatalf("find failed: ok=%v commit=%q", ok, got.Commit)
	}
	if _, ok, _ := s.FindByCommit("zzz"); ok {
		t.Fatal("unexpected hit for missing commit")
	}
}
