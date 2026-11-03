package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const Path = ".git/auteur/config.yaml"

type Privacy struct {
	CapturePrompt    bool `yaml:"capture_prompt"`
	PromptMaxLength  int  `yaml:"prompt_max_length"`
	RedactSecrets    bool `yaml:"redact_secrets"`
	CaptureSessionID bool `yaml:"capture_session_id"`
}

type Config struct {
	Version    int               `yaml:"version"`
	AgentNames map[string]string `yaml:"agent_names,omitempty"`
	Privacy    Privacy           `yaml:"privacy"`
}

func Default() Config {
	return Config{
		Version: 1,
		Privacy: Privacy{
			CapturePrompt:    true,
			PromptMaxLength:  200,
			RedactSecrets:    true,
			CaptureSessionID: true,
		},
		AgentNames: map[string]string{
			"claude": "claude-code",
			"codex":  "codex",
			"cursor": "cursor",
		},
	}
}

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

func Save(repoRoot string, c Config) error {
	p := filepath.Join(repoRoot, Path)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
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
