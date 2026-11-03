package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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

func isIgnored(rel string) bool {
	ignoredDirs := []string{"node_modules/", "dist/", "build/", "vendor/", ".next/", "target/"}
	for _, d := range ignoredDirs {
		if strings.Contains(rel, d) {
			return true
		}
	}
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

			// The snapshot directory holds the "before" state of files the AI has touched
			snapDir := filepath.Join(root, ".git", "auteur", "snapshots")
			os.MkdirAll(snapDir, 0o755)
			snapPath := filepath.Join(snapDir, strings.ReplaceAll(rel, "/", "_"))

			var beforeContent []byte
			if data, err := os.ReadFile(snapPath); err == nil {
				beforeContent = data
			} else {
				// No pre-tool snapshot. This means the PreToolUse hook didn't fire.
				// Fall back to the full file (safe, conservative).
				saveDirty(root, agentName, rel, [][2]int{{1, 999999}}, sessionID, prompt)
				return nil
			}

			afterContent, err := os.ReadFile(filePath)
			if err != nil {
				// File might have been deleted
				return nil
			}

			// Save the new snapshot for next time
			os.WriteFile(snapPath, afterContent, 0o644)

			// Compute the diff between before and after
			diffOut := runDiff(beforeContent, afterContent)
			ranges := git.AddedLines(diffOut)
			if len(ranges) == 0 {
				// No actual changes (e.g. AI read the file)
				return nil
			}

			saveDirty(root, agentName, rel, ranges, sessionID, prompt)
			return nil
		},
	}
	return cmd
}

// runDiff calls `git diff --no-index` on two temp files.
func runDiff(before, after []byte) string {
	tmpDir, _ := os.MkdirTemp("", "auteur-diff-*")
	defer os.RemoveAll(tmpDir)

	oldFile := filepath.Join(tmpDir, "old")
	newFile := filepath.Join(tmpDir, "new")
	os.WriteFile(oldFile, before, 0644)
	os.WriteFile(newFile, after, 0644)

	cmd := exec.Command("git", "diff", "--no-index", "-U0", oldFile, newFile)
	out, _ := cmd.CombinedOutput()

	// git diff --no-index exits 1 if there are differences, which is normal
	// We just return the diff output
	return string(out)
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
