package provenance

import (
	"testing"
	"time"
)

func recWith(commit string, lines ...LineRange) Record {
	return Record{
		Commit:     commit,
		Timestamp:  time.Now().UTC(),
		Confidence: ConfidenceHigh,
		Lines:      lines,
	}
}

func TestLookupLine_ExactHit(t *testing.T) {
	recs := []Record{
		recWith("a", LineRange{File: "a.go", Start: 1, End: 10, Origin: OriginAgent}),
	}
	got := LookupLine(recs, "a.go", 5)
	if !got.Found {
		t.Fatalf("expected hit, got %+v", got)
	}
	if got.Record.Commit != "a" {
		t.Errorf("commit = %q", got.Record.Commit)
	}
}

func TestLookupLine_Boundaries(t *testing.T) {
	recs := []Record{
		recWith("a", LineRange{File: "a.go", Start: 10, End: 20, Origin: OriginAgent}),
	}
	for _, line := range []int{10, 15, 20} {
		if !LookupLine(recs, "a.go", line).Found {
			t.Errorf("line %d should hit", line)
		}
	}
	for _, line := range []int{9, 21} {
		if LookupLine(recs, "a.go", line).Found {
			t.Errorf("line %d should miss", line)
		}
	}
}

func TestLookupLine_MostRecentWins(t *testing.T) {
	recs := []Record{
		recWith("old", LineRange{File: "a.go", Start: 1, End: 100, Origin: OriginHuman}),
		recWith("new", LineRange{File: "a.go", Start: 40, End: 60, Origin: OriginAgent}),
	}
	got := LookupLine(recs, "a.go", 50)
	if !got.Found || got.Record.Commit != "new" {
		t.Fatalf("want commit=new, got %+v", got)
	}
	got = LookupLine(recs, "a.go", 10)
	if !got.Found || got.Record.Commit != "old" {
		t.Fatalf("want commit=old, got %+v", got)
	}
}

func TestLookupLine_PathNormalization(t *testing.T) {
	recs := []Record{
		recWith("a", LineRange{File: "./src/a.go", Start: 1, End: 5, Origin: OriginAgent}),
	}
	for _, q := range []string{"src/a.go", "./src/a.go"} {
		if !LookupLine(recs, q, 3).Found {
			t.Errorf("query %q should hit", q)
		}
	}
}

func TestLookupLine_NoRecord(t *testing.T) {
	got := LookupLine(nil, "a.go", 1)
	if got.Found {
		t.Fatal("expected miss on empty ledger")
	}
	if got.Note == "" {
		t.Error("expected a note explaining the miss")
	}
}

func TestLookupLine_InvalidLine(t *testing.T) {
	got := LookupLine(nil, "a.go", 0)
	if got.Found || got.Note == "" {
		t.Errorf("want invalid-line note, got %+v", got)
	}
}
