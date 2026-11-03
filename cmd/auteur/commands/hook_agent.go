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

			var filePath string
			var claudePayload struct {
				ToolInput struct {
					FilePath string `json:"file_path"`
				} `json:"tool_input"`
			}
			if json.Unmarshal(input, &claudePayload) == nil && claudePayload.ToolInput.FilePath != "" {
				filePath = claudePayload.ToolInput.FilePath
			}
			if filePath == "" {
				var cursorPayload struct {
					FilePath string `json:"filePath"`
				}
				if json.Unmarshal(input, &cursorPayload) == nil && cursorPayload.FilePath != "" {
					filePath = cursorPayload.FilePath
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

			// Try to diff the file to get specific line ranges.
			// If the file is untracked, diff shows the whole file.
			diff, _ := git.Run(root, "diff", "-U0", "HEAD", "--", rel)
			if diff == "" {
				// Maybe it's untracked?
				diff, _ = git.Run(root, "diff", "-U0", "--no-index", "/dev/null", rel)
			}

			ranges := git.AddedLines(diff)
			if len(ranges) == 0 {
				// Fallback if diff is empty (e.g. hook fired but nothing changed)
				ranges = [][2]int{{1, 999999}}
			}

			dirtyList := filepath.Join(root, ".git", "auteur", agentName+"-dirty-files.txt")
			f, err := os.OpenFile(dirtyList, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
			if err == nil {
				for _, r := range ranges {
					fmt.Fprintf(f, "%s:%d:%d\n", rel, r[0], r[1])
				}
				f.Close()
			}
			return nil
		},
	}
	return cmd
}
