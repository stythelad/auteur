package policy

import (
	"fmt"

	"github.com/stythelad/auteur/internal/provenance"
)

type Violation struct {
	Rule   string `json:"rule"`
	File   string `json:"file"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Commit string `json:"commit"`
	Reason string `json:"reason"`
}

// Options controls Check behavior.
type Options struct {
	// SkipCommits are commit SHAs to ignore (the --since base).
	SkipCommits map[string]bool
	// Strict makes unknown-origin lines fail in protected paths.
	// Default true; callers pass false for --no-strict.
	Strict bool
}

// Check evaluates every record against every rule.
func Check(records []provenance.Record, p Policy, opts Options) []Violation {
	var out []Violation
	for _, rec := range records {
		if opts.SkipCommits[rec.Commit] {
			continue
		}
		for _, rule := range p.Rules {
			for _, lr := range rec.Lines {
				if !AnyMatch(rule.Paths, lr.File) {
					continue
				}
				if v, ok := evaluate(rule, lr, rec, opts); ok {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// isAgentLike reports whether an origin counts as agent-authored for
// policy purposes. Mixed contains agent code; treat it as agent.
func isAgentLike(o provenance.Origin) bool {
	return o == provenance.OriginAgent || o == provenance.OriginMixed
}

func evaluate(rule Rule, lr provenance.LineRange, rec provenance.Record, opts Options) (Violation, bool) {
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

	agentLike := isAgentLike(lr.Origin)

	if rule.ForbidAgentAuthored && agentLike {
		return mk("agent-authored lines are forbidden in this path")
	}
	if rule.RequireHumanSignoff && agentLike {
		if len(rec.Review.HumanSignoff) == 0 {
			return mk("agent-authored lines require human signoff; none recorded")
		}
	}
	if rule.RequireCIPassed && agentLike {
		if !rec.Review.CIPassed {
			return mk("agent-authored lines require a passing CI run; none recorded")
		}
	}
	if rule.RequireProvenance && lr.Origin == provenance.OriginUnknown {
		return mk("provenance is unknown in a path that requires it")
	}

	// Default-on strict mode: unknown-origin lines in any rule's paths
	// fail unless the caller opts out.
	if opts.Strict && lr.Origin == provenance.OriginUnknown {
		return mk(fmt.Sprintf("unknown-origin lines flagged in strict mode (rule %s)", rule.Name))
	}

	return Violation{}, false
}

func Summarize(vs []Violation) string {
	if len(vs) == 0 {
		return "no violations"
	}
	return fmt.Sprintf("%d violation(s)", len(vs))
}
