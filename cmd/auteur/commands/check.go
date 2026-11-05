package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

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

			// Reject PRs that modify the trust root.
			if base != "" {
				changedOut, diffErr := git.Run(root, "diff", "--name-only", base+"..HEAD", "--", ".auteur/allowed_signers")
				if diffErr != nil {
					return fmt.Errorf("check trust-root changes against %s: %w", base, diffErr)
				}
				if strings.TrimSpace(changedOut) != "" {
					return fmt.Errorf("security violation: PR modifies the trust root (.auteur/allowed_signers). trust root changes must be merged by an administrator into the base branch")
				}
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

			recs, err := loadRecords(root, trustRoot)
			if err != nil {
				return err
			}
			if len(recs) == 0 && !allowMissingLedger {
				fmt.Fprintln(os.Stderr, "auteur: warning: no provenance records found in notes or local cache. proceeding, but protected paths may fail.")
			}

			// Cryptographic verification always runs, even when --allow-missing-ledger
			// is passed. That flag only skips the "no records" error.
			if err := provenance.VerifyLedger(recs); err != nil {
				return fmt.Errorf("security check failed: %w", err)
			}
			if err := provenance.VerifySignoffs(recs, trustRoot); err != nil {
				return fmt.Errorf("cryptographic verification failed: %w", err)
			}

			// Filter out untrusted signoffs.
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

			// Line-level validation against the actual diff.
			if base != "" {
				diffOut, diffErr := git.Run(root, "diff", "-U0", base+"..HEAD")
				if diffErr != nil {
					return fmt.Errorf("calculate line-level diff against %s: %w", base, diffErr)
				}
				{
					fileLines := parseDiffFiles(diffOut)
					for file, lines := range fileLines {
						for _, lineRange := range lines {
							for _, rule := range pol.Rules {
								if policy.AnyMatch(rule.Paths, file) {
									var uncoveredLines []int
									for lineNum := lineRange[0]; lineNum <= lineRange[1]; lineNum++ {
										covered := false
										for _, r := range recs {
											// Historical records reachable from the base branch must not
											// satisfy provenance requirements for lines introduced by this PR.
											if skipSet[r.Commit] {
												continue
											}
											for _, lr := range r.Lines {
												if lr.File == file && lineNum >= lr.Start && lineNum <= lr.End {
													covered = true
													break
												}
											}
											if covered {
												break
											}
										}
										if !covered {
											uncoveredLines = append(uncoveredLines, lineNum)
										}
									}
									if len(uncoveredLines) > 0 {
										start := uncoveredLines[0]
										prev := start
										for _, l := range uncoveredLines[1:] {
											if l != prev+1 {
												vs = append(vs, policy.Violation{
													Rule:       rule.Name,
													File:       file,
													Start:      start,
													End:        prev,
													Commit:     "UNKNOWN",
													Reason:     "line changed in this PR but has no provenance record",
													IsAdvisory: false,
												})
												start = l
											}
											prev = l
										}
										vs = append(vs, policy.Violation{
											Rule:       rule.Name,
											File:       file,
											Start:      start,
											End:        prev,
											Commit:     "UNKNOWN",
											Reason:     "line changed in this PR but has no provenance record",
											IsAdvisory: false,
										})
									}
									break
								}
							}
						}
					}
				}
			}

			failed := false
			for _, v := range vs {
				if !v.IsAdvisory {
					failed = true
					break
				}
			}

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
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", v.Commit)
					fmt.Fprintf(cmd.OutOrStdout(), "  reason: %s\n", v.Reason)
				}
				if len(vs) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no violations")
				}
			}

			if failed {
				count := 0
				for _, v := range vs {
					if !v.IsAdvisory {
						count++
					}
				}
				return fmt.Errorf("policy check failed: %d violation(s)", count)
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
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// parseDiffFiles extracts file -> []([start, end]) from a unified diff.
func parseDiffFiles(diff string) map[string][][2]int {
	out := map[string][][2]int{}
	lines := strings.Split(diff, "\n")

	var currentFile string
	var lineNum int
	// hunkRegex matches @@ -a,b +c,d @@
	hunkRegex := regexpMustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

	for _, line := range lines {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = line[6:]
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			currentFile = ""
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			matches := hunkRegex.FindStringSubmatch(line)
			if len(matches) > 1 {
				lineNum = parseInt(matches[1])
				if lineNum == 0 {
					lineNum = 1
				}
			}
			continue
		}

		if currentFile == "" {
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			ranges := out[currentFile]
			if len(ranges) > 0 && ranges[len(ranges)-1][1]+1 == lineNum {
				ranges[len(ranges)-1][1] = lineNum
			} else {
				ranges = append(ranges, [2]int{lineNum, lineNum})
			}
			out[currentFile] = ranges
			lineNum++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			// skip
		} else if strings.HasPrefix(line, " ") {
			lineNum++
		}
	}
	return out
}

func regexpMustCompile(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
