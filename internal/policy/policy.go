package policy

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const Path = ".auteur/policy.yaml"

type Rule struct {
	Name                string   `yaml:"name"`
	Paths               []string `yaml:"paths"`
	RequireHumanSignoff bool     `yaml:"require_human_signoff,omitempty"`
	ForbidAgentAuthored bool     `yaml:"forbid_agent_authored,omitempty"`
	RequireCIPassed     bool     `yaml:"require_ci_passed,omitempty"`
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
			},
			{
				Name:                "no-agent-code-in-migrations",
				Paths:               []string{"migrations/**", "**/migrations/**"},
				ForbidAgentAuthored: true,
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
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	return p, nil
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
