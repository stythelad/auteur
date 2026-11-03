package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
)

func NewHookClaudeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook-claude",
		Short: "Internal: Claude Code PostToolUse hook",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return nil // Not in a repo, just ignore
			}

			// Read JSON from stdin
			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			var payload struct {
				ToolName  string `json:"tool_name"`
				ToolInput struct {
					FilePath string `json:"file_path"`
				} `json:"tool_input"`
			}

			if err := json.Unmarshal(input, &payload); err != nil {
				return nil // Invalid JSON, ignore
			}

			if payload.ToolInput.FilePath == "" {
				return nil
			}

			// Make path relative to repo root
			rel, err := filepath.Rel(root, payload.ToolInput.FilePath)
			if err != nil {
				return nil
			}

			// Append to dirty files list
			dirtyList := filepath.Join(root, ".git", "auteur", "claude-dirty-files.txt")
			f, err := os.OpenFile(dirtyList, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				f.WriteString(rel + "\n")
				f.Close()
			}
			return nil
		},
	}
	return cmd
}
