package policy

import (
	"testing"

	"github.com/stythelad/auteur/internal/provenance"
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

func opts(strict bool) Options {
	return Options{Strict: strict}
}

func TestCheck_NoViolationOnHumanLines(t *testing.T) {
	p := Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginHuman))}
	if vs := Check(rs, p, opts(true)); len(vs) != 0 {
		t.Fatalf("unexpected violations: %+v", vs)
	}
}

func TestCheck_AgentNeedsSignoff_NoSignoff(t *testing.T) {
	p := Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginAgent))}
	vs := Check(rs, p, opts(false))
	if len(vs) != 1 || vs[0].Rule != "r" {
		t.Fatalf("want 1 violation, got %+v", vs)
	}
}

func TestCheck_MixedTreatedAsAgent(t *testing.T) {
	p := Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginMixed))}
	vs := Check(rs, p, opts(false))
	if len(vs) != 1 {
		t.Fatalf("mixed should fire signoff rule, got %d", len(vs))
	}
}

func TestCheck_ForbidAgentAuthored(t *testing.T) {
	p := Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"migrations/**"}, ForbidAgentAuthored: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("migrations/001.sql", 1, 1, provenance.OriginAgent))}
	if vs := Check(rs, p, opts(false)); len(vs) != 1 {
		t.Fatalf("want 1, got %d", len(vs))
	}
}

func TestCheck_SkipCommits(t *testing.T) {
	p := Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("old-sha", nil, false, lr("auth/x.go", 1, 5, provenance.OriginAgent))}
	o := Options{SkipCommits: map[string]bool{"old-sha": true}, Strict: true}
	if vs := Check(rs, p, o); len(vs) != 0 {
		t.Fatalf("expected skip, got %+v", vs)
	}
}

func TestCheck_StrictUnknown_FailsByDefault(t *testing.T) {
	p := Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireHumanSignoff: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginUnknown))}
	if vs := Check(rs, p, opts(true)); len(vs) != 1 {
		t.Fatalf("strict default should flag unknown, got %d", len(vs))
	}
	if vs := Check(rs, p, opts(false)); len(vs) != 0 {
		t.Fatalf("non-strict should not flag unknown, got %d", len(vs))
	}
}

func TestCheck_RequireProvenance_FlagsUnknownEvenNonStrict(t *testing.T) {
	p := Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"auth/**"}, RequireProvenance: true}}}
	rs := []provenance.Record{rec("a", nil, false, lr("auth/x.go", 1, 5, provenance.OriginUnknown))}
	if vs := Check(rs, p, opts(false)); len(vs) != 1 {
		t.Fatalf("require_provenance should flag unknown, got %d", len(vs))
	}
}

func TestPolicy_Validate(t *testing.T) {
	cases := []struct {
		name string
		p    Policy
		ok   bool
	}{
		{"valid", Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"a/**"}, RequireHumanSignoff: true}}}, true},
		{"bad version", Policy{Version: 2}, false},
		{"missing name", Policy{Version: 1, Rules: []Rule{{Paths: []string{"a/**"}, RequireHumanSignoff: true}}}, false},
		{"missing paths", Policy{Version: 1, Rules: []Rule{{Name: "r", RequireHumanSignoff: true}}}, false},
		{"no booleans", Policy{Version: 1, Rules: []Rule{{Name: "r", Paths: []string{"a/**"}}}}, false},
		{"dup names", Policy{Version: 1, Rules: []Rule{
			{Name: "r", Paths: []string{"a/**"}, RequireHumanSignoff: true},
			{Name: "r", Paths: []string{"b/**"}, RequireHumanSignoff: true},
		}}, false},
	}
	for _, c := range cases {
		err := c.p.Validate()
		if (err == nil) != c.ok {
			t.Errorf("%s: ok=%v err=%v", c.name, c.ok, err)
		}
	}
}
