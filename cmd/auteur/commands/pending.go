package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

func NewPendingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pending",
		Short: "Inspect pending provenance records",
	}
	cmd.AddCommand(newPendingListCmd(), newPendingShowCmd(), NewPendingGCCmd())
	return cmd
}

func newPendingListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List pending records",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := repoRoot()
			if err != nil {
				return err
			}
			ps, err := provenance.ListPending(root)
			if err != nil {
				return err
			}
			if len(ps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no pending records")
				return nil
			}
			for _, p := range ps {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  files=%d ranges=%d\n",
					p.UUID, p.CreatedAt.Format("2006-01-02T15:04:05Z"),
					countFiles(p.Record.Lines), len(p.Record.Lines))
			}
			return nil
		},
	}
}

func newPendingShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <uuid>",
		Short: "Show a single pending record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := repoRoot()
			if err != nil {
				return err
			}
			p, err := provenance.LoadPending(root, args[0])
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(p)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uuid:    %s\n", p.UUID)
			fmt.Fprintf(cmd.OutOrStdout(), "created: %s\n", p.CreatedAt.Format("2006-01-02T15:04:05Z"))
			fmt.Fprintf(cmd.OutOrStdout(), "command: %v\n", p.Command)
			fmt.Fprintf(cmd.OutOrStdout(), "lines:\n")
			for _, r := range p.Record.Lines {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %d-%d  %s\n", r.File, r.Start, r.End, r.Origin)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "output JSON")
	return cmd
}

func repoRoot() (string, error) {
	return git.RepoRoot(".")
}
