package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

func NewBlameCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "blame <file>:<line>",
		Short: "Show provenance for a single file:line",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, line, err := parseTarget(args[0])
			if err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if !git.IsRepo(cwd) {
				return fmt.Errorf("%s is not inside a git repository", cwd)
			}
			root, err := git.RepoRoot(cwd)
			if err != nil {
				return err
			}

			file = filepath.Clean(file)
			if filepath.IsAbs(file) {
				rel, relErr := filepath.Rel(root, file)
				if relErr == nil {
					file = rel
				}
			}

			store := provenance.Open(root)
			recs, err := store.Load()
			if err != nil {
				return err
			}

			att := provenance.LookupLine(recs, file, line)

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(att)
			}

			printAttribution(cmd.OutOrStdout(), file, line, att)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output the attribution as JSON")
	return cmd
}

func parseTarget(s string) (string, int, error) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", 0, fmt.Errorf("expected <file>:<line>, got %q", s)
	}
	path := s[:i]
	n, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return "", 0, fmt.Errorf("bad line number in %q: %w", s, err)
	}
	if path == "" {
		return "", 0, fmt.Errorf("empty path in %q", s)
	}
	return path, n, nil
}

func printAttribution(w interface{ Write([]byte) (int, error) }, file string, line int, a provenance.Attribution) {
	if !a.Found {
		fmt.Fprintf(w, "%s:%d\n", file, line)
		fmt.Fprintf(w, "  origin:  unknown\n")
		fmt.Fprintf(w, "  note:    %s\n", a.Note)
		return
	}

	r := a.Record
	fmt.Fprintf(w, "%s:%d\n", file, line)
	if r.Agent != nil && r.Agent.Name != "" {
		label := r.Agent.Name
		if r.Agent.Model != "" {
			label += " (" + r.Agent.Model + ")"
		}
		fmt.Fprintf(w, "  author:  %s\n", label)
	} else {
		fmt.Fprintf(w, "  author:  %s <%s>\n", r.Author.Name, r.Author.Email)
	}
	if r.Agent != nil && r.Agent.SessionID != "" {
		fmt.Fprintf(w, "  session: %s\n", r.Agent.SessionID)
	}
	if r.Agent != nil && r.Agent.PromptHash != "" {
		fmt.Fprintf(w, "  prompt:  %s\n", shortHash(r.Agent.PromptHash))
	}
	fmt.Fprintf(w, "  commit:  %s  (%s)\n", shortSHA(r.Commit), r.Timestamp.Format("2006-01-02"))
	fmt.Fprintf(w, "  range:   %d-%d\n", a.Range.Start, a.Range.End)
	fmt.Fprintf(w, "  origin:  %s   [confidence: %s]\n", a.Range.Origin, r.Confidence)
	if len(r.Review.HumanSignoff) > 0 {
		fmt.Fprintf(w, "  review:  signoff by %s\n", strings.Join(r.Review.HumanSignoff, ", "))
	}
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func shortHash(s string) string {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		s = s[i+1:]
	}
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
