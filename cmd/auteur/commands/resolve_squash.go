package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/notes"
)

func NewResolveSquashCmd() *cobra.Command {
	var originalSHAs []string
	var fromMessage bool

	cmd := &cobra.Command{
		Use:   "resolve-squash",
		Short: "Copy notes from original commits to a squash commit",
		Long: `When GitHub squash-merges a PR, the original commits become orphaned.
This command finds commits with notes and copies them to the current HEAD.

It supports two modes:
  - --origin <sha> (repeatable): explicitly list the original commit SHAs
  - --from-message: parse "Auteur-Origin-SHA: <sha>" trailers from the current HEAD commit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			head, err := git.HeadSHA(root)
			if err != nil {
				return err
			}

			var shas []string

			if len(originalSHAs) > 0 {
				shas = originalSHAs
			} else if fromMessage {
				msg, err := git.Run(root, "log", "-1", "--format=%B", head)
				if err != nil {
					return err
				}
				for _, line := range strings.Split(msg, "\n") {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Auteur-Origin-SHA:") {
						sha := strings.TrimSpace(strings.TrimPrefix(line, "Auteur-Origin-SHA:"))
						if sha != "" {
							shas = append(shas, sha)
						}
					}
				}
			}

			if len(shas) == 0 {
				// Auto-detect: find notes that don't match any current commit
				allNotes, err := notes.ListAllRecords(root)
				if err != nil {
					return err
				}
				// Check which note SHAs still exist as commits
				for _, rec := range allNotes {
					exists, _ := git.Run(root, "cat-file", "-e", rec.Commit+"^{commit}")
					if exists == "" {
						shas = append(shas, rec.Commit)
					}
				}
			}

			if len(shas) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No squash candidates found.")
				return nil
			}

			copied := 0
			for _, origSHA := range shas {
				recs, found, err := notes.ReadRecords(root, origSHA)
				if err != nil {
					return err
				}
				if !found {
					continue
				}
				for _, rec := range recs {
					rec.Commit = head
					if err := notes.WriteRecord(root, head, rec); err != nil {
						return fmt.Errorf("write note for %s: %w", origSHA[:7], err)
					}
					copied++
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Copied notes from %s to %s\n", origSHA[:7], head[:7])
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Resolved %d squash records to %s\n", copied, head[:7])
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&originalSHAs, "origin", nil, "Original commit SHA to copy from (repeatable)")
	cmd.Flags().BoolVar(&fromMessage, "from-message", false, "Parse Auteur-Origin-SHA trailers from HEAD commit message")
	return cmd
}
