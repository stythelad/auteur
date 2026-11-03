package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
)

func NewHookAgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook-agent <agent-name>",
		Short: "Internal: Generic post-tool hook for IDE agents",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			agentName := args[0]
			root, err := git.RepoRoot(".")
			if err != nil {
				return nil // Not in a repo, ignore
			}

			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			// Try to parse the various JSON formats (Claude, Cursor, etc.)
			var filePath string

			// Format 1: Claude Code ({"tool_input": {"file_path": "..."}})
			var claudePayload struct {
				ToolInput struct {
					FilePath string `json:"file_path"`
				} `json:"tool_input"`
			}
			if json.Unmarshal(input, &claudePayload) == nil && claudePayload.ToolInput.FilePath != "" {
				filePath = claudePayload.ToolInput.FilePath
			}

			// Format 2: Cursor ({"filePath": "..."})
			if filePath == "" {
				var cursorPayload struct {
					FilePath string `json:"filePath"`
				}
				if json.Unmarshal(input, &cursorPayload) == nil && cursorPayload.FilePath != "" {
					filePath = cursorPayload.FilePath
				}
			}

			// Format 3: Just a raw path in stdin
			if filePath == "" {
				raw := strings.TrimSpace(string(input))
				if !strings.HasPrefix(raw, "{") && raw != "" {
					filePath = raw
				}
			}

			if filePath == "" {
				return nil // Nothing to do
			}

			// Make path relative to repo root
			rel, err := filepath.Rel(root, filePath)
			if err != nil {
				return nil
			}

			// Append to dirty files list
			dirtyList := filepath.Join(root, ".git", "auteur", agentName+"-dirty-files.txt")
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
