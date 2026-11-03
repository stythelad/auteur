package commands

import (
	"github.com/spf13/cobra"
)

// Root returns the top-level sentinel command.
func Root() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auteur",
		Short: "Provenance ledger for AI-authored code",
		Long: `Auteur records which lines of a codebase were written by humans,
which by agents, and under what conditions — so that git blame can
answer "was this written by a model?"`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(NewInitCmd(), NewBlameCmd(), NewWrapCmd(), NewPendingCmd(), NewResolveCmd(), NewCheckCmd(), NewScanHistoryCmd(), NewRewriteCmd(), NewSignCmd(), NewHookClaudeCmd())
	return cmd
}
