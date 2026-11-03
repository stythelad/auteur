package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/config"
	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/policy"
	"github.com/stythelad/auteur/internal/provenance"
)

func NewInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize an Auteur provenance ledger in the current repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if !git.IsRepo(cwd) {
				return fmt.Errorf("%s is not inside a git repository", cwd)
			}
			root, err := git.RepoRoot(cwd)
			if err != nil {
				return err
			}

			store := provenance.Open(root)
			if force {
				_ = os.Remove(store.Path())
			}
			if err := store.Init(); err != nil {
				return err
			}

			cfgPath := root + "/" + config.Path
			if _, statErr := os.Stat(cfgPath); statErr == nil && !force {
				fmt.Fprintf(cmd.OutOrStdout(), "config already exists at %s (use --force to overwrite)\n", cfgPath)
			} else {
				if err := config.Save(root, config.Default()); err != nil {
					return err
				}
			}

			policyPath := root + "/" + policy.Path
			if _, pErr := os.Stat(policyPath); os.IsNotExist(pErr) {
				os.MkdirAll(filepath.Dir(policyPath), 0o755)
				if sErr := policy.Save(root, policy.Default()); sErr != nil {
					return sErr
				}
			}

			pcInstalled, prInstalled, hErr := installHooks(root, force)
			if hErr != nil {
				return hErr
			}

			fmt.Fprintf(cmd.OutOrStdout(), "initialized Auteur at %s\n", root)
			fmt.Fprintf(cmd.OutOrStdout(), "  ledger: %s\n", store.Path())
			fmt.Fprintf(cmd.OutOrStdout(), "  config: %s\n", cfgPath)
			hookStatus := func(installed bool) string {
				if installed {
					return "installed"
				}
				return "up to date"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  hook:   .git/hooks/post-commit (%s)\n", hookStatus(pcInstalled))
			fmt.Fprintf(cmd.OutOrStdout(), "  hook:   .git/hooks/post-rewrite (%s)\n", hookStatus(prInstalled))

			// Write Claude Code settings
			claudeDir := filepath.Join(root, ".claude")
			if err := os.MkdirAll(claudeDir, 0o755); err == nil {
				claudeSettings := `{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": "auteur hook-agent claude-code"
          }
        ]
      }
    ]
  }
}`
				os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(claudeSettings), 0o644)
				fmt.Fprintf(cmd.OutOrStdout(), "  claude: .claude/settings.json (installed)\n")
			}

			// Write Cursor settings
			cursorDir := filepath.Join(root, ".cursor")
			if err := os.MkdirAll(cursorDir, 0o755); err == nil {
				cursorSettings := `{
  "hooks": {
    "afterFileEdit": [
      {
        "command": "auteur hook-agent cursor"
      }
    ]
  }
}`
				os.WriteFile(filepath.Join(cursorDir, "hooks.json"), []byte(cursorSettings), 0o644)
				fmt.Fprintf(cmd.OutOrStdout(), "  cursor: .cursor/hooks.json (installed)\n")
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing ledger, config, and hooks")
	return cmd
}
