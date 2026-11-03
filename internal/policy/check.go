package policy

import (
	"fmt"

	"github.com/stythelad/auteur/internal/provenance"
)

type Violation struct {
	Rule       string `json:"rule"`
	File       string `json:"file"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Commit     string `json:"commit"`
	Reason     string `json:"reason"`
	IsAdvisory bool   `json:"is_advisory"`
}

type Options struct {
	SkipCommits map[string]bool
	Strict      bool
}

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

func isAgentLike(o provenance.Origin) bool {
	return o == provenance.OriginAgent || o == provenance.OriginMixed
}

// confidenceValue maps confidence to a number for comparison.
func confidenceValue(c provenance.Confidence) int {
	switch c {
	case provenance.ConfidenceHigh:
		return 3
	case provenance.ConfidenceMedium:
		return 2
	case provenance.ConfidenceLow:
		return 1
	default:
		return 0
	}
}

// requiresFail decides if a violation should fail the build based on the
// rule's min_confidence setting.
func requiresFail(rule Rule, rec provenance.Record) bool {
	if rule.MinConfidence == "" {
		return true // default is hard fail
	}
	req := confidenceValue(provenance.Confidence(rule.MinConfidence))
	actual := confidenceValue(rec.Confidence)
	return actual >= req
}

func evaluate(rule Rule, lr provenance.LineRange, rec provenance.Record, opts Options) (Violation, bool) {
	mk := func(reason string) (Violation, bool) {
		return Violation{
			Rule:       rule.Name,
			File:       lr.File,
			Start:      lr.Start,
			End:        lr.End,
			Commit:     rec.Commit,
			Reason:     reason,
			IsAdvisory: !requiresFail(rule, rec),
		}, true
	}

	agentLike := isAgentLike(lr.Origin)

	if rule.ForbidAgentAuthored && agentLike {
		return mk("agent-authored lines are forbidden in this path")
	}
	if rule.RequireHumanSignoff && agentLike {
		if len(rec.Review.Signoffs) == 0 {
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

	if opts.Strict && lr.Origin == provenance.OriginUnknown {
		return mk(fmt.Sprintf("unknown-origin lines flagged in strict mode (rule %s)", rule.Name))
	}

	return Violation{}, false
}
