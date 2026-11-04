package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/notes"
)

func NewPushCmd() *cobra.Command {
	var remote string
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Push provenance notes to the remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}
			if err := notes.Push(root, remote); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pushed refs/notes/%s to %s\n", notes.NotesRef, remote)
			return nil
		},
	}
	cmd.Flags().StringVar(&remote, "remote", "origin", "Git remote to push to")
	return cmd
}

func NewFetchCmd() *cobra.Command {
	var remote string
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch provenance notes from the remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}
			if err := notes.Fetch(root, remote); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Fetched refs/notes/%s from %s\n", notes.NotesRef, remote)
			return nil
		},
	}
	cmd.Flags().StringVar(&remote, "remote", "origin", "Git remote to fetch from")
	return cmd
}
