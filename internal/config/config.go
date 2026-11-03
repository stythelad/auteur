package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stythelad/auteur/internal/git"

	"gopkg.in/yaml.v3"
)

const Path = ".git/auteur/config.yaml"
const TrustRootPath = ".auteur/allowed_signers"

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

type TrustRoot struct {
	TrustedSigners map[string]string `yaml:"trusted_signers"`
}

func Default() Config {
	return Config{
		Version: 1,
		Privacy: Privacy{
			CapturePrompt:    false, // Opt-in by default
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

func LoadTrustRoot(repoRoot string) (map[string]string, error) {
	// Try to read from the working tree first
	path := filepath.Join(repoRoot, TrustRootPath)
	raw, err := os.ReadFile(path)
	if err == nil {
		var tr TrustRoot
		if err := yaml.Unmarshal(raw, &tr); err != nil {
			return nil, fmt.Errorf("parse trust root: %w", err)
		}
		return tr.TrustedSigners, nil
	}

	// If the file is missing, fail closed
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil // Return nil map to indicate "no trust root"
	}

	// If it exists but can't be read (e.g. it's a directory), fail
	return nil, fmt.Errorf("read trust root %s: %w", TrustRootPath, err)
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

// LoadTrustRootFromGit reads .auteur/allowed_signers from a specific git ref.
// Used in CI to read the trust root from the base branch, not the PR head.
func LoadTrustRootFromGit(repoRoot, ref string) (map[string]string, error) {
	raw, err := git.Run(repoRoot, "show", ref+":"+TrustRootPath)
	if err != nil {
		// Fail explicitly if the base branch does not have a trust root.
		return nil, fmt.Errorf("base branch %q does not contain %s: %w", ref, TrustRootPath, err)
	}

	var tr TrustRoot
	if err := yaml.Unmarshal([]byte(raw), &tr); err != nil {
		return nil, fmt.Errorf("parse trust root from %s: %w", ref, err)
	}
	return tr.TrustedSigners, nil
}
