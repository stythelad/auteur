package commands

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/pokhrelmishan/sentinel/internal/git"
	"github.com/pokhrelmishan/sentinel/internal/provenance"
)

func NewWrapCmd() *cobra.Command {
	var agentName, model, session string
	cmd := &cobra.Command{
		Use:   "wrap -- <command> [args...]",
		Short: "Run a command and record the line ranges it changed",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			beforeSHA, err := git.StashCreate(root)
			if err != nil {
				return fmt.Errorf("snapshot before: %w", err)
			}
			beforeUntracked, err := git.UntrackedFiles(root)
			if err != nil {
				return fmt.Errorf("list untracked before: %w", err)
			}

			child := exec.Command(args[0], args[1:]...)
			child.Dir = cwd
			child.Stdin = os.Stdin
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			runErr := child.Run()

			afterSHA, err := git.StashCreate(root)
			if err != nil {
				return fmt.Errorf("snapshot after: %w", err)
			}
			afterUntracked, err := git.UntrackedFiles(root)
			if err != nil {
				return fmt.Errorf("list untracked after: %w", err)
			}

			var diffText string
			switch {
			case beforeSHA == "" && afterSHA == "":
				diffText = ""
			case beforeSHA == "":
				diffText, err = git.DiffFrom(root, "HEAD")
			case afterSHA == "":
				diffText, err = git.DiffFrom(root, beforeSHA)
			default:
				diffText, err = git.Run(root, "diff", beforeSHA, afterSHA)
			}
			if err != nil {
				return fmt.Errorf("diff snapshots: %w", err)
			}

			newFiles := diffStringSet(afterUntracked, beforeUntracked)
			for _, f := range newFiles {
				fileDiff, dErr := git.RunAllowExit1(root, "diff", "--no-index", "/dev/null", f)
				if dErr != nil {
					continue
				}
				diffText += "\n" + fileDiff
			}

			ranges := provenance.ParseUnifiedDiff(diffText)
			if len(ranges) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no changes detected; nothing recorded")
				if runErr != nil {
					return runErr
				}
				return nil
			}

			uuid, err := newUUID()
			if err != nil {
				return err
			}

			rec := provenance.Record{
				Commit:     "pending:" + uuid,
				Timestamp:  time.Now().UTC(),
				Confidence: provenance.ConfidenceHigh,
				Lines:      ranges,
			}
			if agentName != "" || model != "" || session != "" {
				rec.Agent = &provenance.Agent{
					Name:      agentName,
					Model:     model,
					SessionID: session,
				}
			}

			p := provenance.Pending{
				UUID:      uuid,
				CreatedAt: time.Now().UTC(),
				Command:   args,
				BeforeSHA: beforeSHA,
				AfterSHA:  afterSHA,
				Record:    rec,
			}
			if err := provenance.WritePending(root, p); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "recorded pending %s\n", uuid)
			fmt.Fprintf(cmd.OutOrStdout(), "  files: %d  line ranges: %d\n", countFiles(ranges), len(ranges))
			fmt.Fprintf(cmd.OutOrStdout(), "  view: sentinel pending show %s\n", uuid)

			if runErr != nil {
				return runErr
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agentName, "agent", "", "agent name (e.g. claude-code)")
	cmd.Flags().StringVar(&model, "model", "", "model identifier (e.g. claude-opus-5.5)")
	cmd.Flags().StringVar(&session, "session", "", "session id for correlation")
	return cmd
}

func newUUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func diffStringSet(after, before []string) []string {
	set := map[string]bool{}
	for _, f := range before {
		set[f] = true
	}
	var out []string
	for _, f := range after {
		if !set[f] {
			out = append(out, f)
		}
	}
	return out
}

func countFiles(rs []provenance.LineRange) int {
	set := map[string]bool{}
	for _, r := range rs {
		set[r.File] = true
	}
	return len(set)
}
