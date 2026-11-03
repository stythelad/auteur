package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/stythelad/auteur/internal/config"
	"github.com/stythelad/auteur/internal/git"
	"github.com/stythelad/auteur/internal/provenance"
)

func NewVerifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Audit the cryptographic integrity and authenticity of the ledger",
		Long: `Runs a deep audit of the provenance ledger. It checks:
1. Integrity: Is the SHA-256 hash chain unbroken?
2. Authenticity: Are all SSH signatures valid?
3. Identity: Who signed each agent commit?`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			store := provenance.Open(root)
			recs, err := store.Load()
			if err != nil {
				return err
			}

			integrityErr := provenance.VerifyLedger(recs)
			cfg, _ := config.Load(root)
			authErr := provenance.VerifySignoffs(recs, cfg.TrustedSigners)

			fmt.Fprintf(cmd.OutOrStdout(), "=== Auteur Ledger Audit ===\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Total records: %d\n\n", len(recs))

			fmt.Fprintf(cmd.OutOrStdout(), "[1] Integrity (SHA-256 Hash Chain)\n")
			if integrityErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Status: FAILED\n  Error: %v\n\n", integrityErr)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  Status: PASSED\n  The ledger has not been tampered with.\n\n")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "[2] Authenticity (SSH Signatures)\n")
			if authErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  Status: FAILED\n  Error: %v\n\n", authErr)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  Status: PASSED\n  All cryptographic signatures are valid.\n\n")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "[3] Signoff Roster\n")
			signedCount := 0
			for _, r := range recs {
				if len(r.Review.Signoffs) > 0 {
					signedCount++
					var signers []string
					for _, s := range r.Review.Signoffs {
						signers = append(signers, s.Identity)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", r.Commit[:7], strings.Join(signers, ", "))
				}
			}
			if signedCount == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  No commits have been cryptographically signed yet.\n")
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n=== Trust Model ===\n")
			fmt.Fprintf(cmd.OutOrStdout(), "- The hash chain proves this ledger has not been accidentally corrupted or hand-edited.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "- The SSH signatures prove which local keys authorised which exact code changes.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "- Note: This tool assumes the signing keys are not compromised. It does not replace CODEOWNERS or human review.\n")

			if integrityErr != nil || authErr != nil {
				return fmt.Errorf("audit failed")
			}
			return nil
		},
	}
	return cmd
}
