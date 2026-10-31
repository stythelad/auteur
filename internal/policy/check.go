package policy

import (
	"fmt"

	"github.com/pokhrelmishan/sentinel/internal/provenance"
)

type Violation struct {
	Rule   string `json:"rule"`
	File   string `json:"file"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Commit string `json:"commit"`
	Reason string `json:"reason"`
}

// Check evaluates every record against every rule.
// skipCommits is a set of commit SHAs to ignore (the --since base).
// strict additionally flags unknown-origin lines under rule paths.
func Check(records []provenance.Record, p Policy, skipCommits map[string]bool, strict bool) []Violation {
	var out []Violation
	for _, rec := range records {
		if skipCommits[rec.Commit] {
			continue
		}
		for _, rule := range p.Rules {
			for _, lr := range rec.Lines {
				if !AnyMatch(rule.Paths, lr.File) {
					continue
				}
				if v, ok := evaluate(rule, lr, rec, strict); ok {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

func evaluate(rule Rule, lr provenance.LineRange, rec provenance.Record, strict bool) (Violation, bool) {
	mk := func(reason string) (Violation, bool) {
		return Violation{
			Rule:   rule.Name,
			File:   lr.File,
			Start:  lr.Start,
			End:    lr.End,
			Commit: rec.Commit,
			Reason: reason,
		}, true
	}

	if rule.ForbidAgentAuthored && lr.Origin == provenance.OriginAgent {
		return mk("agent-authored lines are forbidden in this path")
	}

	if rule.RequireHumanSignoff && lr.Origin == provenance.OriginAgent {
		if len(rec.Review.HumanSignoff) == 0 {
			return mk("agent-authored lines require human signoff; none recorded")
		}
	}

	if rule.RequireCIPassed && lr.Origin == provenance.OriginAgent {
		if !rec.Review.CIPassed {
			return mk("agent-authored lines require a passing CI run; none recorded")
		}
	}

	if strict && lr.Origin == provenance.OriginUnknown {
		return mk(fmt.Sprintf("unknown-origin lines flagged in strict mode (rule %s)", rule.Name))
	}

	return Violation{}, false
}

// Summarize returns a short human summary of violations, grouped by rule.
func Summarize(vs []Violation) string {
	if len(vs) == 0 {
		return "no violations"
	}
	return fmt.Sprintf("%d violation(s)", len(vs))
}
