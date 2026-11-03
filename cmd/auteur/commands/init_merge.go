package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// mergeClaudeSettings merges the Auteur hooks into an existing Claude settings.json
func mergeClaudeSettings(path string) error {
	var cfg map[string]interface{}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("existing claude settings.json is invalid: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	hooksMap, _ := cfg["hooks"].(map[string]interface{})
	if hooksMap == nil {
		hooksMap = map[string]interface{}{}
		cfg["hooks"] = hooksMap
	}

	// Helper to append a hook if it doesn't exist
	appendHook := func(eventType, matcher, command string) {
		events, _ := hooksMap[eventType].([]interface{})

		// Check if this command already exists
		for _, ev := range events {
			evMap, _ := ev.(map[string]interface{})
			evHooks, _ := evMap["hooks"].([]interface{})
			for _, h := range evHooks {
				hMap, _ := h.(map[string]interface{})
				if hMap["command"] == command {
					return // Already installed
				}
			}
		}

		// Add new hook
		newHook := map[string]interface{}{
			"matcher": matcher,
			"hooks": []interface{}{
				map[string]interface{}{
					"type":    "command",
					"command": command,
				},
			},
		}
		hooksMap[eventType] = append(events, newHook)
	}

	appendHook("PreToolUse", "Write|Edit", "auteur hook-agent-pre claude-code")
	appendHook("PostToolUse", "Write|Edit", "auteur hook-agent claude-code")

	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, raw, 0o644)
}

// mergeCursorHooks merges the Auteur hook into an existing Cursor hooks.json
func mergeCursorHooks(path string) error {
	var cfg map[string]interface{}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("existing cursor hooks.json is invalid: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if cfg == nil {
		cfg = map[string]interface{}{}
	}

	hooksMap, _ := cfg["hooks"].(map[string]interface{})
	if hooksMap == nil {
		hooksMap = map[string]interface{}{}
		cfg["hooks"] = hooksMap
	}

	events, _ := hooksMap["afterFileEdit"].([]interface{})

	// Check if our command exists
	for _, ev := range events {
		evMap, _ := ev.(map[string]interface{})
		if evMap["command"] == "auteur hook-agent cursor" {
			return writeJSON(path, cfg) // Already exists
		}
	}

	// Add new hook
	hooksMap["afterFileEdit"] = append(events, map[string]interface{}{
		"command": "auteur hook-agent cursor",
	})

	return writeJSON(path, cfg)
}

func writeJSON(path string, cfg map[string]interface{}) error {
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	return os.WriteFile(path, raw, 0o644)
}
