package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/config"
	"github.com/stythelad/auteur/internal/git"
)

var secretRegex = regexp.MustCompile(`(sk-[a-zA-Z0-9]{20,}|ghp_[a-zA-Z0-9]{20,}|AKIA[0-9A-Z]{16})`)

func sanitizePrompt(cfg config.Config, prompt string) string {
	if prompt == "" {
		return ""
	}
	if !cfg.Privacy.CapturePrompt {
		h := sha256.Sum256([]byte(prompt))
		return "sha256:" + hex.EncodeToString(h[:])
	}
	if cfg.Privacy.RedactSecrets {
		prompt = secretRegex.ReplaceAllString(prompt, "[REDACTED]")
	}
	if cfg.Privacy.PromptMaxLength > 0 && len(prompt) > cfg.Privacy.PromptMaxLength {
		prompt = prompt[:cfg.Privacy.PromptMaxLength] + "..."
	}
	return prompt
}

// isIgnored checks if a file is likely generated, binary, or huge.
func isIgnored(rel string) bool {
	ignoredDirs := []string{"node_modules/", "dist/", "build/", "vendor/", ".next/", "target/"}
	for _, d := range ignoredDirs {
		if strings.Contains(rel, d) {
			return true
		}
	}
	// Check extension
	binExts := []string{".png", ".jpg", ".jpeg", ".gif", ".ico", ".pdf", ".zip", ".tar", ".gz", ".exe", ".dll", ".so", ".dylib", ".lock", "-lock.json", "-lock.yaml"}
	for _, e := range binExts {
		if strings.HasSuffix(rel, e) {
			return true
		}
	}
	return false
}

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

			cfg, _ := config.Load(root)

			input, err := io.ReadAll(os.Stdin)
			if err != nil {
				return err
			}

			var filePath, sessionID, prompt string

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

			if !cfg.Privacy.CaptureSessionID {
				sessionID = ""
			}
			prompt = sanitizePrompt(cfg, prompt)

			rel, err := filepath.Rel(root, filePath)
			if err != nil {
				return nil
			}

			if isIgnored(rel) {
				// For generated files, just attribute the whole file
				ranges := [][2]int{{1, 999999}}
				saveDirty(root, agentName, rel, ranges, sessionID, prompt)
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

			saveDirty(root, agentName, rel, ranges, sessionID, prompt)
			return nil
		},
	}
	return cmd
}

func saveDirty(root, agent, rel string, ranges [][2]int, session, prompt string) {
	dirtyList := filepath.Join(root, ".git", "auteur", agent+"-dirty-files.txt")
	f, err := os.OpenFile(dirtyList, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		for _, r := range ranges {
			fmt.Fprintf(f, "%s:%d:%d:%s:%s\n", rel, r[0], r[1], session, prompt)
		}
		f.Close()
	}
}
