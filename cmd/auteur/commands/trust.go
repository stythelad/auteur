package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/stythelad/auteur/internal/git"
)

func NewTrustCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust",
		Short: "Manage the committed trust root (.auteur/allowed_signers)",
	}
	cmd.AddCommand(NewTrustAddCmd())
	return cmd
}

func NewTrustAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <identity> <pubkey-path>",
		Short: "Add a trusted signer to the committed trust root",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}

			identity := args[0]
			pubPath := args[1]

			// Read the pubkey
			pubBytes, err := os.ReadFile(pubPath)
			if err != nil {
				return fmt.Errorf("read pubkey: %w", err)
			}
			pubParts := strings.Fields(string(pubBytes))
			if len(pubParts) < 2 {
				return fmt.Errorf("invalid pubkey format in %s", pubPath)
			}
			pubKey := pubParts[0] + " " + pubParts[1]

			// Load or create the trust file
			trustPath := filepath.Join(root, ".auteur", "allowed_signers")
			type TrustFile struct {
				TrustedSigners map[string]string `yaml:"trusted_signers"`
			}
			var tf TrustFile
			tf.TrustedSigners = map[string]string{}

			if raw, err := os.ReadFile(trustPath); err == nil {
				yaml.Unmarshal(raw, &tf)
				if tf.TrustedSigners == nil {
					tf.TrustedSigners = map[string]string{}
				}
			}

			tf.TrustedSigners[identity] = pubKey

			// Write it back
			raw, err := yaml.Marshal(tf)
			if err != nil {
				return err
			}
			os.MkdirAll(filepath.Dir(trustPath), 0o755)
			if err := os.WriteFile(trustPath, raw, 0o644); err != nil {
				return fmt.Errorf("write trust file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added %s to .auteur/allowed_signers\n", identity)
			fmt.Fprintf(cmd.OutOrStdout(), "Remember to commit this file!\n")
			return nil
		},
	}
	return cmd
}
