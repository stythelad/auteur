package commands

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

func NewSignCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sign [commit-sha]",
		Short: "Approve agent-authored code for a specific commit",
		Long: `Creates a git commit containing an "Auteur-Signoff: <sha>" trailer,
and records the git author's identity in the provenance ledger.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			targetSHA := "HEAD"
			if len(args) > 0 {
				targetSHA = args[0]
			}

			// Resolve the short SHA to the full SHA
			fullTargetSHA, err := git.Run(root, "rev-parse", targetSHA)
			if err != nil {
				return fmt.Errorf("could not resolve commit %q: %w", targetSHA, err)
			}

			// 1. Create an empty commit with the signoff trailer.
			// This forces Git to attach the committer's identity to the action.
			msg := fmt.Sprintf("chore: signoff on %s\n\nAuteur-Signoff: %s\n", fullTargetSHA[:7], fullTargetSHA)
			commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", msg)
			commitCmd.Dir = root
			out, err := commitCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to create signoff commit: %v\n%s", err, out)
			}

			// 2. Extract the identity from the new commit
			signerName, err := git.Run(root, "log", "-1", "--format=%an")
			if err != nil {
				return err
			}
			signerEmail, err := git.Run(root, "log", "-1", "--format=%ae")
			if err != nil {
				return err
			}
			identity := fmt.Sprintf("%s <%s>", signerName, signerEmail)

			// 3. Update the ledger for the target commit
			store := provenance.Open(root)
			recs, err := store.Load()
			if err != nil {
				return err
			}

			found := false
			for i := range recs {
				if recs[i].Commit == fullTargetSHA {
					// Check if already signed
					for _, s := range recs[i].Review.HumanSignoff {
						if s == identity {
							fmt.Fprintf(cmd.OutOrStdout(), "already signed by %s\n", identity)
							return nil
						}
					}
					recs[i].Review.HumanSignoff = append(recs[i].Review.HumanSignoff, identity)
					found = true
				}
			}

			if !found {
				return fmt.Errorf("no provenance record found for commit %s", fullTargetSHA)
			}

			// Rewrite the ledger with the new signoff
			if err := store.Rewrite(recs); err != nil {
				return fmt.Errorf("failed to write ledger: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully signed off on %s as %s\n", fullTargetSHA[:7], identity)
			fmt.Fprintf(cmd.OutOrStdout(), "A signoff commit was created. Push this commit and the ledger to share the signoff.\n")
			return nil
		},
	}
	return cmd
}
