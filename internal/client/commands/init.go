package commands

import (
	"encoding/base64"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devsync/devsync/internal/client/keystore"
	"github.com/devsync/devsync/internal/crypto"
)

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a device keypair and back up the private key",
		RunE: func(_ *cobra.Command, _ []string) error {
			if keystore.Exists() && !force {
				return fmt.Errorf("device key already exists; use --force to overwrite (DESTROYS old key)")
			}
			pass, err := promptPassphrase("Choose a passphrase for this device key: ")
			if err != nil {
				return err
			}
			confirm, err := promptPassphrase("Confirm passphrase: ")
			if err != nil {
				return err
			}
			if pass != confirm {
				return fmt.Errorf("passphrases do not match")
			}
			if len(pass) < 8 {
				return fmt.Errorf("passphrase must be at least 8 characters")
			}

			kp, err := crypto.GenerateKeyPair()
			if err != nil {
				return err
			}
			ek, err := crypto.EncryptPrivateKey(kp, pass)
			if err != nil {
				return err
			}
			if err := keystore.Save(ek); err != nil {
				return err
			}

			fp := crypto.Fingerprint(kp.SignPub)
			// Forced backup: show the private-key material and instruct the user.
			backup := base64.StdEncoding.EncodeToString(kp.SignPriv)
			fmt.Println("\n=== DEVICE KEY CREATED ===")
			fmt.Printf("Fingerprint: %s\n", fp)
			fmt.Println("\n!!! BACK UP THIS PRIVATE KEY NOW — there is NO recovery without it !!!")
			fmt.Println("Store it in a password manager or offline. Anyone with it + your")
			fmt.Println("passphrase can impersonate this device.")
			fmt.Println()
			fmt.Printf("PRIVATE KEY (backup): %s\n", backup)
			fmt.Println("\nNext: `devsync config set server_url <url>` then `devsync register`.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing device key")
	return cmd
}
