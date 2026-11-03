package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
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

			// Detect renames in this commit to map old paths to new paths
			renameMap := map[string]string{}
			diffTree, _ := git.Run(root, "diff-tree", "-M", "-r", "--name-status", head+"^", head)
			for _, line := range splitLines(diffTree) {
				parts := strings.Split(line, "	")
				if len(parts) == 3 && strings.HasPrefix(parts[0], "R") {
					renameMap[parts[1]] = parts[2]
				}
			}

			// Process agent dirty files
			agentFiles := []string{"claude-code", "cursor"}
			for _, agent := range agentFiles {
				dirtyList := filepath.Join(root, ".git", "auteur", agent+"-dirty-files.txt")
				var agentMeta *provenance.Agent
				if dirtyBytes, err := os.ReadFile(dirtyList); err == nil {
					var lines []provenance.LineRange
					for _, line := range splitLines(string(dirtyBytes)) {
						// Format: file:start:end:session:prompt
						parts := strings.SplitN(line, ":", 5)
						if len(parts) >= 3 {
							file := parts[0]
							start, _ := strconv.Atoi(parts[1])
							end, _ := strconv.Atoi(parts[2])
							if newName, ok := renameMap[file]; ok {
								file = newName
							}
							if files[file] {
								sessionID := ""
								prompt := ""
								if len(parts) >= 4 {
									sessionID = parts[3]
								}
								if len(parts) >= 5 {
									prompt = parts[4]
								}

								// Save metadata to the record
								lines = append(lines, provenance.LineRange{File: file, Start: start, End: end, Origin: provenance.OriginAgent})
								if sessionID != "" || prompt != "" {
									if agentMeta == nil {
										agentMeta = &provenance.Agent{Name: agent}
									}
									agentMeta.SessionID = sessionID
									// For prompt, we ideally hash it, but we'll store a snippet for now.
									if prompt != "" {
										agentMeta.PromptHash = prompt
									}
								}
							}
						}
					}
					if len(lines) > 0 {
						if agentMeta == nil {
							agentMeta = &provenance.Agent{Name: agent}
						}
						rec := provenance.Record{
							Commit:     head,
							Timestamp:  time.Now().UTC(),
							Confidence: provenance.ConfidenceHigh,
							Agent:      agentMeta,
							Lines:      lines,
						}
						provenance.Open(root).Append(rec)
					}
					os.Remove(dirtyList) // Clear after processing
				}
			}

			// Detect cherry-picks to preserve attribution
			msg, _ := git.CommitMessage(root, head)
			if strings.Contains(msg, "(cherry picked from commit ") {
				start := strings.Index(msg, "(cherry picked from commit ") + 26
				end := strings.Index(msg[start:], ")")
				if end != -1 {
					origSHA := msg[start : start+end]
					// Copy records from origSHA to head
					store := provenance.Open(root)
					recs, _ := store.Load()
					for _, r := range recs {
						if r.Commit == origSHA {
							c := r
							c.Commit = head
							c.Timestamp = time.Now().UTC()
							store.Append(c)
						}
					}
					if verbose {
						fmt.Fprintf(cmd.OutOrStdout(), "preserved cherry-pick attribution from %s\n", origSHA[:7])
					}
				}
			}

			// Clear snapshots after commit, since the next AI edit starts fresh
			snapDir := filepath.Join(root, ".git", "auteur", "snapshots")
			os.RemoveAll(snapDir)

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
