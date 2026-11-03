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

			// Merge Claude Code settings
			claudePath := filepath.Join(root, ".claude", "settings.json")
			if err := mergeClaudeSettings(claudePath); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  claude: ERROR: %v\n", err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  claude: .claude/settings.json (merged)\n")
			}

			// Merge Cursor settings
			cursorPath := filepath.Join(root, ".cursor", "hooks.json")
			if err := mergeCursorHooks(cursorPath); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  cursor: ERROR: %v\n", err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  cursor: .cursor/hooks.json (merged)\n")
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing ledger, config, and hooks")
	return cmd
}
