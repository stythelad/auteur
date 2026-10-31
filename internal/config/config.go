package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const Path = ".auteur/config.yaml"

// Config is the on-disk Sentinel configuration.
// v1 fields only; policy lives in a separate file later.
type Config struct {
	Version int `yaml:"version"`
	// AgentNames maps a detected agent identifier to a friendly name.
	// Reserved for the heuristic fallback in a later step.
	AgentNames map[string]string `yaml:"agent_names,omitempty"`
}

// Default returns the config written by `sentinel init`.
func Default() Config {
	return Config{
		Version: 1,
		AgentNames: map[string]string{
			"claude": "claude-code",
			"codex":  "codex",
			"cursor": "cursor",
		},
	}
}

// Load reads the config from repoRoot, returning defaults if missing.
func Load(repoRoot string) (Config, error) {
	p := filepath.Join(repoRoot, Path)
	raw, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return c, nil
}

// Save writes the config to repoRoot.
func Save(repoRoot string, c Config) error {
	p := filepath.Join(repoRoot, Path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create .auteur dir: %w", err)
	}
	raw, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
