package commands

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/config"
	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/notes"
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

			// Check if the key is trusted before signing (FAIL CLOSED)
			pubKey, err := provenance.GetPublicKey(sshKey)
			if err != nil {
				return fmt.Errorf("read public key: %w", err)
			}
			trustRoot, err := config.LoadTrustRoot(root)
			if err != nil {
				return fmt.Errorf("load trust root: %w", err)
			}

			// If the trust root is missing or empty, no one is authorized.
			if len(trustRoot) == 0 {
				return fmt.Errorf("no trusted signers configured. you must run `auteur trust add %s %s.pub` and commit `.auteur/allowed_signers` before signing", identity, sshKey)
			}

			trustedKey, ok := trustRoot[identity]
			if !ok || trustedKey != pubKey {
				return fmt.Errorf("your public key is not in the trusted signers list. run `auteur trust add %s %s.pub` first", identity, sshKey)
			}

			store := provenance.Open(root)
			recs, err := store.Load()
			if err != nil {
				return err
			}

			var signedRecs []provenance.Record
			for i := range recs {
				if recs[i].Commit == fullTargetSHA {
					if err := provenance.SignRecord(&recs[i], identity, sshKey); err != nil {
						return fmt.Errorf("failed to sign record: %w", err)
					}
					signedRecs = append(signedRecs, recs[i])
				}
			}

			if len(signedRecs) == 0 {
				return fmt.Errorf("no provenance record found for commit %s", fullTargetSHA)
			}

			if err := store.Rewrite(recs); err != nil {
				return fmt.Errorf("failed to write ledger: %w", err)
			}

			// Write the updated (signed) records to notes so CI can see them.
			for _, rec := range signedRecs {
				if err := notes.WriteRecord(root, rec.Commit, rec); err != nil {
					return fmt.Errorf("failed to write note for %s: %w", rec.Commit[:7], err)
				}
			}

			// Create an empty Git commit with the signoff trailer, SSH-signed.
			msg := fmt.Sprintf("chore: signoff on %s\n\nAuteur-Signoff: %s\n", fullTargetSHA[:7], fullTargetSHA)
			commitCmd := exec.Command("git", "commit", "--allow-empty", "-S", "-m", msg)
			commitCmd.Dir = root
			out, err := commitCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("ledger updated, but failed to create signed git commit (make sure gpg.format=ssh and user.signingkey are set): %v\n%s", err, out)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully cryptographically signed off on %s as %s\n", fullTargetSHA[:7], identity)
			fmt.Fprintf(cmd.OutOrStdout(), "Created SSH-signed Git commit with signoff trailer.\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&sshKey, "ssh-key", "", "Path to SSH private key (default: ~/.ssh/id_ed25519)")
	return cmd
}
