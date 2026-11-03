package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/config"
	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/policy"
	"github.com/stythelad/auteur/internal/provenance"
)

func NewCheckCmd() *cobra.Command {
	var (
		asJSON             bool
		since              string
		prBase             string
		noStrict           bool
		allowMissingLedger bool
		ciBase             string
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
			} else if ciBase != "" {
				base = ciBase
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

			store := provenance.Open(root)
			ledgerMissing := !fileExists(store.Path())
			recs, err := store.Load()
			if err != nil {
				return err
			}
			if ledgerMissing && !allowMissingLedger {
				return errors.New("provenance ledger not found; run `auteur init` and `auteur scan-history`, or pass --allow-missing-ledger")
			}

			if !allowMissingLedger {
				var trustRoot map[string]string
				if ciBase != "" {
					trustRoot, err = config.LoadTrustRootFromGit(root, ciBase)
					if err != nil {
						return fmt.Errorf("load trust root from CI base: %w", err)
					}
				} else {
					trustRoot, err = config.LoadTrustRoot(root)
					if err != nil {
						return fmt.Errorf("load trust root: %w", err)
					}
				}

				if err := provenance.VerifyLedger(recs); err != nil {
					return fmt.Errorf("security check failed: %w", err)
				}
				if err := provenance.VerifySignoffs(recs, trustRoot); err != nil {
					return fmt.Errorf("cryptographic verification failed: %w", err)
				}

				// Filter out untrusted signoffs before policy evaluation.
				for i := range recs {
					if len(trustRoot) == 0 {
						recs[i].Review.Signoffs = nil
					} else {
						var trusted []provenance.Signoff
						for _, s := range recs[i].Review.Signoffs {
							if key, ok := trustRoot[s.Identity]; ok && key == s.PublicKey {
								trusted = append(trusted, s)
							}
						}
						recs[i].Review.Signoffs = trusted
					}
				}
			}

			var pol policy.Policy
			if ciBase != "" {
				pol, err = policy.LoadFromGit(root, ciBase)
				if err != nil {
					return fmt.Errorf("load policy from CI base: %w", err)
				}
			} else {
				pol, err = policy.Load(root)
				if err != nil {
					return err
				}
			}

			vs := policy.Check(recs, pol, policy.Options{
				SkipCommits: skipSet,
				Strict:      !noStrict,
			})

			// Check for files changed in the PR that have no provenance record.
			if base != "" {
				changedFilesOut, err := git.Run(root, "diff", "--name-only", base+"..HEAD")
				if err == nil {
					changedFiles := splitLines(changedFilesOut)
					for _, file := range changedFiles {
						hasRecord := false
						for _, r := range recs {
							for _, lr := range r.Lines {
								if lr.File == file {
									hasRecord = true
									break
								}
							}
							if hasRecord {
								break
							}
						}
						if !hasRecord {
							for _, rule := range pol.Rules {
								if policy.AnyMatch(rule.Paths, file) {
									vs = append(vs, policy.Violation{
										Rule:       rule.Name,
										File:       file,
										Start:      0,
										End:        0,
										Commit:     "UNKNOWN",
										Reason:     "file was changed in this PR but has no provenance record (unknown origin)",
										IsAdvisory: false,
									})
									break
								}
							}
						}
					}
				}
			}

			failed := false
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(vs); err != nil {
					return err
				}
			} else {
				for _, v := range vs {
					if v.IsAdvisory {
						fmt.Fprintf(cmd.OutOrStdout(), "ADVISORY   %s:%d-%d  [%s]\n", v.File, v.Start, v.End, v.Rule)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "VIOLATION  %s:%d-%d  [%s]\n", v.File, v.Start, v.End, v.Rule)
						failed = true
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", v.Commit)
					fmt.Fprintf(cmd.OutOrStdout(), "  reason: %s\n", v.Reason)
				}
				if len(vs) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no violations")
				}
			}

			if !asJSON && failed {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&since, "since", "", "only check commits not reachable from this ref")
	cmd.Flags().StringVar(&prBase, "pr-base", "", "shorthand for --since origin/<branch>")
	cmd.Flags().BoolVar(&noStrict, "no-strict", false, "do not fail on unknown-origin lines")
	cmd.Flags().BoolVar(&allowMissingLedger, "allow-missing-ledger", false, "pass even if ledger is missing")
	cmd.Flags().StringVar(&ciBase, "ci-base", "", "Read trust root from this git ref (e.g., origin/main) for CI")
	return cmd
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
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
