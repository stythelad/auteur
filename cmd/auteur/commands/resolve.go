package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pokhrelmishan/auteur/internal/git"
	"github.com/pokhrelmishan/auteur/internal/provenance"
)

func NewResolveCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:   "resolve-pending",
		Short: "Resolve pending records to the current HEAD commit",
		Long: `Called automatically by the post-commit git hook. Matches
pending records against the files changed in HEAD and appends resolved
records to the ledger.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}
			head, err := git.HeadSHA(root)
			if err != nil {
				return err
			}
			files, err := git.CommitFiles(root, head)
			if err != nil {
				return err
			}
			ts, _ := git.CommitTime(root, head)
			n, err := provenance.ResolvePending(root, head, ts, files)
			if err != nil {
				return err
			}
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "resolved %d pending record(s) to %s\n", n, head[:7])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print resolution summary")
	return cmd
}

func NewPendingGCCmd() *cobra.Command {
	var olderThan time.Duration
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Delete pending records older than a threshold",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}
			ps, err := provenance.ListPending(root)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-olderThan)
			n := 0
			for _, p := range ps {
				if p.CreatedAt.Before(cutoff) {
					if err := provenance.DeletePending(root, p.UUID); err != nil {
						return err
					}
					n++
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d pending record(s)\n", n)
			return nil
		},
	}
	cmd.Flags().DurationVar(&olderThan, "older-than", 7*24*time.Hour, "prune pending records older than this")
	return cmd
}
