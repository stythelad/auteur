package policy

import (
	"testing"

	"github.com/pokhrelmishan/auteur/internal/provenance"
)

func rec(commit string, signoff []string, ci bool, lines ...provenance.LineRange) provenance.Record {
	return provenance.Record{
		Commit: commit,
		Review: provenance.Review{HumanSignoff: signoff, CIPassed: ci},
		Lines:  lines,
	}
}

func lr(file string, start, end int, origin provenance.Origin) provenance.LineRange {
	return provenance.LineRange{File: file, Start: start, End: end, Origin: origin}
}

func TestCheck_NoViolationOnHumanLines(t *testing.T) {
	p := Policy{Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginHuman))}
	if vs := Check(rs, p, nil, false); len(vs) != 0 {
		t.Fatalf("unexpected violations: %+v", vs)
	}
}

func TestCheck_RequireHumanSignoff_NoSignoff(t *testing.T) {
	p := Policy{Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginAgent))}
	vs := Check(rs, p, nil, false)
	if len(vs) != 1 || vs[0].Rule != "r" {
		t.Fatalf("want 1 violation, got %+v", vs)
	}
}

func TestCheck_RequireHumanSignoff_WithSignoff(t *testing.T) {
	p := Policy{Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("a", []string{"alice"}, false, lr("auth/x.go", 1, 5, provenance.OriginAgent))}
	if vs := Check(rs, p, nil, false); len(vs) != 0 {
		t.Fatalf("unexpected: %+v", vs)
	}
}

func TestCheck_ForbidAgentAuthored(t *testing.T) {
	p := Policy{Rules: []Rule{{Name: "r", Paths: []string{"migrations/**"}, ForbidAgentAuthored: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("migrations/001.sql", 1, 1, provenance.OriginAgent))}
	if vs := Check(rs, p, nil, false); len(vs) != 1 {
		t.Fatalf("want 1, got %d", len(vs))
	}
}

func TestCheck_SkipCommits(t *testing.T) {
	p := Policy{Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("old-sha", nil, false, lr("auth/x.go", 1, 5, provenance.OriginAgent))}
	vs := Check(rs, p, map[string]bool{"old-sha": true}, false)
	if len(vs) != 0 {
		t.Fatalf("expected skip, got %+v", vs)
	}
}

func TestCheck_StrictUnknown(t *testing.T) {
	p := Policy{Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginUnknown))}
	if vs := Check(rs, p, nil, false); len(vs) != 0 {
		t.Fatalf("non-strict should not flag unknown")
	}
	if vs := Check(rs, p, nil, true); len(vs) != 1 {
		t.Fatalf("strict should flag unknown, got %d", len(vs))
	}
}
