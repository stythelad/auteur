package policy

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const Path = ".git/auteur/policy.yaml"

type Rule struct {
	Name                string   `yaml:"name"`
	Paths               []string `yaml:"paths"`
	RequireHumanSignoff bool     `yaml:"require_human_signoff,omitempty"`
	ForbidAgentAuthored bool     `yaml:"forbid_agent_authored,omitempty"`
	RequireCIPassed     bool     `yaml:"require_ci_passed,omitempty"`
	RequireProvenance   bool     `yaml:"require_provenance,omitempty"`
	// MinConfidence is "high", "medium", or "low". Anything below this
	// is treated as an advisory violation (warn, don't fail).
	MinConfidence string `yaml:"min_confidence,omitempty"`
}

type Policy struct {
	Version int    `yaml:"version"`
	Rules   []Rule `yaml:"rules"`
}

func Default() Policy {
	return Policy{
		Version: 1,
		Rules: []Rule{
			{
				Name:                "protected-paths-need-signoff",
				Paths:               []string{"auth/**", "payments/**", "**/crypto/**"},
				RequireHumanSignoff: true,
				MinConfidence:       "high", // Only fail on wrap-level confidence
			},
			{
				Name:                "no-agent-code-in-migrations",
				Paths:               []string{"migrations/**", "**/migrations/**"},
				ForbidAgentAuthored: true,
				MinConfidence:       "high",
			},
		},
	}
}

func Load(repoRoot string) (Policy, error) {
	raw, err := os.ReadFile(repoRoot + "/" + Path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Policy{}, fmt.Errorf("read policy: %w", err)
	}
	var p Policy
	dec := yaml.NewDecoder(bytesReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	if err := p.Validate(); err != nil {
		return Policy{}, fmt.Errorf("invalid policy: %w", err)
	}
	return p, nil
}

func (p Policy) Validate() error {
	if p.Version != 1 {
		return fmt.Errorf("unsupported policy version %d (want 1)", p.Version)
	}
	seen := map[string]bool{}
	for i, r := range p.Rules {
		if r.Name == "" {
			return fmt.Errorf("rule %d: name is required", i)
		}
		if seen[r.Name] {
			return fmt.Errorf("rule %q: duplicate name", r.Name)
		}
		seen[r.Name] = true
		if len(r.Paths) == 0 {
			return fmt.Errorf("rule %q: paths must be non-empty", r.Name)
		}
		if !r.RequireHumanSignoff && !r.ForbidAgentAuthored && !r.RequireCIPassed && !r.RequireProvenance {
			return fmt.Errorf("rule %q: at least one of require_human_signoff, forbid_agent_authored, require_ci_passed, require_provenance must be true", r.Name)
		}
		if r.MinConfidence != "" && r.MinConfidence != "high" && r.MinConfidence != "medium" && r.MinConfidence != "low" {
			return fmt.Errorf("rule %q: min_confidence must be high, medium, or low", r.Name)
		}
	}
	return nil
}

func Save(repoRoot string, p Policy) error {
	raw, err := yaml.Marshal(p)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	if err := os.WriteFile(repoRoot+"/"+Path, raw, 0o644); err != nil {
		return fmt.Errorf("write policy: %w", err)
	}
	return nil
}
