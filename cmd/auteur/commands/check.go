package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/pokhrelmishan/auteur/internal/git"
	"github.com/pokhrelmishan/auteur/internal/policy"
	"github.com/pokhrelmishan/auteur/internal/provenance"
)

func NewCheckCmd() *cobra.Command {
	var (
		asJSON bool
		since  string
		prBase string
		strict bool
	)
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run the policy against the provenance ledger",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			base := since
			if prBase != "" {
				base = "origin/" + prBase
			}

			skipSet := map[string]bool{}
			if base != "" {
				out, revErr := git.Run(root, "rev-list", base)
				if revErr != nil {
					return fmt.Errorf("list base commits: %w", revErr)
				}
				for _, sha := range splitLines(out) {
					skipSet[sha] = true
				}
			}

			recs, err := provenance.Open(root).Load()
			if err != nil {
				return err
			}

			pol, err := policy.Load(root)
			if err != nil {
				return err
			}

			vs := policy.Check(recs, pol, skipSet, strict)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(vs); err != nil {
					return err
				}
			} else {
				for _, v := range vs {
					fmt.Fprintf(cmd.OutOrStdout(), "VIOLATION  %s:%d-%d  [%s]\n", v.File, v.Start, v.End, v.Rule)
					fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", v.Commit)
					fmt.Fprintf(cmd.OutOrStdout(), "  reason: %s\n", v.Reason)
				}
				if len(vs) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no violations")
				}
			}

			if len(vs) > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&since, "since", "", "only check commits not reachable from this ref")
	cmd.Flags().StringVar(&prBase, "pr-base", "", "shorthand for --since origin/<branch>")
	cmd.Flags().BoolVar(&strict, "strict", false, "flag unknown-origin lines in protected paths")
	return cmd
}

func splitLines(s string) []string {
	var out []string
	for _, line := range splitAny(s, '\n') {
		line = trimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func splitAny(s string, sep rune) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
