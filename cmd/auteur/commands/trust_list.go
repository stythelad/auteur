package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/stythelad/auteur/internal/git"
)

type TrustFile struct {
	TrustedSigners map[string]string `yaml:"trusted_signers"`
}

func loadTrustFile(root string) (*TrustFile, string, error) {
	trustPath := filepath.Join(root, ".auteur", "allowed_signers")
	tf := &TrustFile{TrustedSigners: map[string]string{}}
	raw, err := os.ReadFile(trustPath)
	if err != nil {
		if os.IsNotExist(err) {
			return tf, trustPath, nil
		}
		return nil, "", err
	}
	if err := yaml.Unmarshal(raw, tf); err != nil {
		return nil, "", err
	}
	if tf.TrustedSigners == nil {
		tf.TrustedSigners = map[string]string{}
	}
	return tf, trustPath, nil
}

func NewTrustListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all trusted signers",
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}
			tf, _, err := loadTrustFile(root)
			if err != nil {
				return err
			}
			if len(tf.TrustedSigners) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No trusted signers configured.")
				return nil
			}
			for id, key := range tf.TrustedSigners {
				// Truncate the key for display
				if len(key) > 40 {
					key = key[:40] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", id, key)
			}
			return nil
		},
	}
}

func NewTrustRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <identity>",
		Short: "Remove a signer from the trust root",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := git.RepoRoot(".")
			if err != nil {
				return err
			}
			tf, trustPath, err := loadTrustFile(root)
			if err != nil {
				return err
			}
			identity := args[0]
			if _, ok := tf.TrustedSigners[identity]; !ok {
				return fmt.Errorf("signer %q not found in trust root", identity)
			}
			delete(tf.TrustedSigners, identity)
			raw, err := yaml.Marshal(tf)
			if err != nil {
				return err
			}
			if err := os.WriteFile(trustPath, raw, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %q from trust root. Remember to commit this change.\n", identity)
			return nil
		},
	}
}
