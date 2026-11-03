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

func NewHookAgentPreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook-agent-pre <agent-name>",
		Short: "Internal: Pre-tool hook to snapshot file state before edit",
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

			realPath, err := safeRelPath(root, filePath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "auteur: refusing to snapshot outside repo: %v\n", err)
				return nil
			}

			content, err := secureReadFile(realPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "auteur: error reading file securely: %v\n", err)
				return nil
			}

			snapDir := filepath.Join(root, ".git", "auteur", "snapshots")
			os.MkdirAll(snapDir, 0o755)
			rel, _ := filepath.Rel(root, realPath)
			snapPath := snapshotPath(snapDir, rel)
			os.WriteFile(snapPath, content, 0o644)
			_ = agentName

			return nil
		},
	}
	return cmd
}
