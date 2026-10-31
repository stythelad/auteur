package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/pokhrelmishan/sentinel/internal/git"
	"github.com/pokhrelmishan/sentinel/internal/heuristic"
	"github.com/pokhrelmishan/sentinel/internal/provenance"
)

func NewScanHistoryCmd() *cobra.Command {
	var (
		asJSON        bool
		dryRun        bool
		base          string
		limit         int
		includeMerges bool
	)
	cmd := &cobra.Command{
		Use:   "scan-history",
		Short: "Bootstrap attribution from git commit trailers",
		Long: `Walks git history and records agent-authored line ranges for commits
that mention a known agent in a Co-authored-by trailer. Records are
written with confidence: medium (inferred, not recorded by wrap).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			existing, err := provenance.Open(root).Load()
			if err != nil {
				return err
			}
			inLedger := map[string]bool{}
			for _, r := range existing {
				inLedger[r.Commit] = true
			}

			commits, err := git.CommitList(root, base, limit, includeMerges)
			if err != nil {
				return err
			}

			store := provenance.Open(root)

			var (
				scanned, matched, recorded, already, skipped int
				records                                      []provenance.Record
			)

			for _, sha := range commits {
				scanned++
				if inLedger[sha] {
					already++
					continue
				}
				msg, err := git.CommitMessage(root, sha)
				if err != nil {
					return err
				}
				match := heuristic.DetectAgent(msg)
				if match == nil {
					continue
				}
				matched++

				diff, err := git.CommitDiff(root, sha)
				if err != nil {
					return err
				}
				ranges := provenance.ParseUnifiedDiff(diff)
				if len(ranges) == 0 {
					skipped++
					continue
				}

				name, email, _ := git.CommitAuthor(root, sha)
				tsStr, _ := git.CommitTime(root, sha)
				ts, _ := time.Parse(time.RFC3339, tsStr)

				rec := provenance.Record{
					Commit:     sha,
					Timestamp:  ts,
					Author:     provenance.Author{Name: name, Email: email},
					Agent:      &provenance.Agent{Name: match.Name},
					Confidence: provenance.ConfidenceMedium,
					Lines:      ranges,
				}
				records = append(records, rec)

				if dryRun {
					fmt.Fprintf(cmd.OutOrStdout(), "COMMIT  %s  %s  agent=%s  files=%d  ranges=%d\n",
						shortSHA(sha), ts.Format("2006-01-02"), match.Name,
						countFiles(ranges), len(ranges))
					continue
				}
				if err := store.Append(rec); err != nil {
					return err
				}
				recorded++
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(records)
			}

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would record %d records (dry run; nothing written)\n", len(records))
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "scanned %d commits\n", scanned)
			fmt.Fprintf(cmd.OutOrStdout(), "matched %d agent commits\n", matched)
			fmt.Fprintf(cmd.OutOrStdout(), "recorded %d records (confidence: medium)\n", recorded)
			fmt.Fprintf(cmd.OutOrStdout(), "skipped %d already in ledger\n", already)
			fmt.Fprintf(cmd.OutOrStdout(), "skipped %d with no extractable ranges\n", skipped)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output matched records as JSON")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be recorded, write nothing")
	cmd.Flags().StringVar(&base, "base", "HEAD", "walk commits reachable from this ref")
	cmd.Flags().IntVar(&limit, "limit", 0, "max commits to scan (0 = no limit)")
	cmd.Flags().BoolVar(&includeMerges, "include-merges", false, "include merge commits")
	return cmd
}
