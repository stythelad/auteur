package provenance

import (
	"testing"
)

func TestParseRewritePairs(t *testing.T) {
	in := "aaa bbb\nccc ddd\n\nbadline\n  eee   fff  \n"
	got := ParseRewritePairs(in)
	want := map[string]string{"aaa": "bbb", "ccc": "ddd", "eee": "fff"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q want %q", k, got[k], v)
		}
	}
}

func TestRewriteRecords_CopiesToNewSHA(t *testing.T) {
	s, dir := newTempStore(t)

	r := sampleRecord()
	r.Commit = "old"
	if err := s.Append(r); err != nil {
		t.Fatal(err)
	}

	n, _, err := RewriteRecords(dir, map[string]string{"old": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 copy, got %d", n)
	}

	recs, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (old + new), got %d", len(recs))
	}
	if recs[0].Commit != "old" || recs[1].Commit != "new" {
		t.Fatalf("unexpected commits: %q, %q", recs[0].Commit, recs[1].Commit)
	}
	if len(recs[1].Lines) != len(r.Lines) {
		t.Fatalf("line ranges not preserved")
	}
}

func TestRewriteRecords_SameSHANoOp(t *testing.T) {
	s, dir := newTempStore(t)
	r := sampleRecord()
	r.Commit = "same"
	_ = s.Append(r)

	n, _, err := RewriteRecords(dir, map[string]string{"same": "same"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 copies, got %d", n)
	}
}

func TestRewriteRecords_MissingSourceNoOp(t *testing.T) {
	_, dir := newTempStore(t)
	n, _, err := RewriteRecords(dir, map[string]string{"nonexistent": "newsha"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}
