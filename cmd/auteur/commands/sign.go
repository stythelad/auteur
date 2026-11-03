package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/config"
	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

func NewSignCmd() *cobra.Command {
	var sshKey string
	cmd := &cobra.Command{
		Use:   "sign [commit-sha]",
		Short: "Cryptographically approve agent-authored code",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			targetSHA := "HEAD"
			if len(args) > 0 {
				targetSHA = args[0]
			}
			fullTargetSHA, err := git.Run(root, "rev-parse", targetSHA)
			if err != nil {
				return fmt.Errorf("could not resolve commit %q: %w", targetSHA, err)
			}

			identity, err := git.Run(root, "config", "user.name")
			if err != nil || identity == "" {
				return fmt.Errorf("git user.name is not set")
			}

			// Check if the key is trusted before signing
			pubKey, err := provenance.GetPublicKey(sshKey)
			if err != nil {
				return fmt.Errorf("read public key: %w", err)
			}
			trustRoot, err := config.LoadTrustRoot(root)
			if err != nil {
				return fmt.Errorf("load trust root: %w", err)
			}
			if len(trustRoot) > 0 {
				trustedKey, ok := trustRoot[identity]
				if !ok || trustedKey != pubKey {
					return fmt.Errorf("your public key is not in the trusted signers list. run `auteur trust add %s %s.pub` first", identity, sshKey)
				}
			}

			store := provenance.Open(root)
			recs, err := store.Load()
			if err != nil {
				return err
			}

			found := false
			for i := range recs {
				if recs[i].Commit == fullTargetSHA {
					if err := provenance.SignRecord(&recs[i], identity, sshKey); err != nil {
						return fmt.Errorf("failed to sign record: %w", err)
					}
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("no provenance record found for commit %s", fullTargetSHA)
			}

			if err := store.Rewrite(recs); err != nil {
				return fmt.Errorf("failed to write ledger: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully cryptographically signed off on %s as %s\n", fullTargetSHA[:7], identity)
			return nil
		},
	}
	cmd.Flags().StringVar(&sshKey, "ssh-key", "", "Path to SSH private key (default: ~/.ssh/id_ed25519)")
	return cmd
}
