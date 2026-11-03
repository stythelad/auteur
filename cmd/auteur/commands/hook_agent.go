package commands

import (
	"encoding/json"
	"fmt"
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
				return nil
			}

			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			var filePath, sessionID, prompt string

			// Try Claude JSON
			var claudePayload struct {
				SessionID string `json:"session_id"`
				Prompt    string `json:"prompt"`
				ToolInput struct {
					FilePath string `json:"file_path"`
				} `json:"tool_input"`
			}
			if json.Unmarshal(input, &claudePayload) == nil && claudePayload.ToolInput.FilePath != "" {
				filePath = claudePayload.ToolInput.FilePath
				sessionID = claudePayload.SessionID
				prompt = claudePayload.Prompt
			}

			// Try Cursor JSON
			if filePath == "" {
				var cursorPayload struct {
					FilePath  string `json:"filePath"`
					SessionID string `json:"sessionId"`
					Prompt    string `json:"prompt"`
				}
				if json.Unmarshal(input, &cursorPayload) == nil && cursorPayload.FilePath != "" {
					filePath = cursorPayload.FilePath
					sessionID = cursorPayload.SessionID
					prompt = cursorPayload.Prompt
				}
			}

			if filePath == "" {
				raw := strings.TrimSpace(string(input))
				if !strings.HasPrefix(raw, "{") && raw != "" {
					filePath = raw
				}
			}

			if filePath == "" {
				return nil
			}

			rel, err := filepath.Rel(root, filePath)
			if err != nil {
				return nil
			}

			diff, _ := git.Run(root, "diff", "-U0", "HEAD", "--", rel)
			if diff == "" {
				diff, _ = git.Run(root, "diff", "-U0", "--no-index", "/dev/null", rel)
			}
			ranges := git.AddedLines(diff)
			if len(ranges) == 0 {
				ranges = [][2]int{{1, 999999}}
			}

			// Save to dirty file including metadata
			dirtyList := filepath.Join(root, ".git", "auteur", agentName+"-dirty-files.txt")
			f, err := os.OpenFile(dirtyList, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				for _, r := range ranges {
					// Format: file:start:end:session:prompt
					fmt.Fprintf(f, "%s:%d:%d:%s:%s\n", rel, r[0], r[1], sessionID, prompt)
				}
				f.Close()
			}
			return nil
		},
	}
	return cmd
}
