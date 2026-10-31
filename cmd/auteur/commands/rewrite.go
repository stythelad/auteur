package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/pokhrelmishan/auteur/internal/git"
	"github.com/pokhrelmishan/auteur/internal/provenance"
)

func NewRewriteCmd() *cobra.Command {
	var (
		fromStdin bool
		verbose   bool
	)
	cmd := &cobra.Command{
		Use:   "rewrite [old-sha new-sha]",
		Short: "Copy attribution records from old SHAs to rewritten SHAs",
		Long: `Called by the post-rewrite git hook after rebase, amend, or other
history-rewriting operations. Can also be invoked manually:

    auteur rewrite <old-sha> <new-sha>

Or, from a post-rewrite-style stdin stream:

    auteur rewrite --stdin < pairs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			var pairs map[string]string
			if fromStdin {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return err
				}
				pairs = provenance.ParseRewritePairs(string(b))
			} else {
				if len(args) != 2 {
					return fmt.Errorf("expected 2 args (old new) or --stdin")
				}
				pairs = map[string]string{args[0]: args[1]}
			}

			n, err := provenance.RewriteRecords(root, pairs)
			if err != nil {
				return err
			}
			if verbose {
				fmt.Fprintf(cmd.OutOrStdout(), "rewrote %d record(s) across %d pair(s)\n", n, len(pairs))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read 'old new' pairs from stdin")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print summary")
	return cmd
}
