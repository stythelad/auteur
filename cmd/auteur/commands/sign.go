package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pokhrelmishan/auteur/internal/git"
	"github.com/pokhrelmishan/auteur/internal/provenance"
)

func NewSignCmd() *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use:   "sign [commit-sha]",
		Short: "Approve agent-authored code for a specific commit",
		Long: `Adds your name to the human_signoff array for the specified commit
in the provenance ledger. If no SHA is provided, defaults to HEAD.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			sha := "HEAD"
			if len(args) > 0 {
				sha = args[0]
			}

			// Resolve the short SHA to the full SHA
			fullSHA, err := git.Run(root, "rev-parse", sha)
			if err != nil {
				return fmt.Errorf("could not resolve commit %q: %w", sha, err)
			}

			if user == "" {
				user, err = git.Run(root, "config", "user.name")
				if err != nil || user == "" {
					return fmt.Errorf("could not determine your git user.name; pass --user")
				}
			}

			store := provenance.Open(root)
			recs, err := store.Load()
			if err != nil {
				return err
			}

			found := false
			for i := range recs {
				if recs[i].Commit == fullSHA {
					// Check if already signed
					for _, s := range recs[i].Review.HumanSignoff {
						if s == user {
							fmt.Fprintf(cmd.OutOrStdout(), "already signed by %s\n", user)
							return nil
						}
					}
					recs[i].Review.HumanSignoff = append(recs[i].Review.HumanSignoff, user)
					found = true
				}
			}

			if !found {
				return fmt.Errorf("no provenance record found for commit %s (short: %s)", fullSHA, sha)
			}

			// Rewrite the ledger (this is the one place we don't append, we rewrite)
			if err := store.Rewrite(recs); err != nil {
				return fmt.Errorf("failed to write ledger: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully signed off on %s as %s\n", fullSHA[:7], user)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "Name to record (defaults to git user.name)")
	return cmd
}
