package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
)

func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Generate a device keypair and recovery phrase",
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

			seed, err := crypto.GenerateRecoverySeed()
			if err != nil {
				return err
			}
			kp, err := crypto.DeriveKeyPairFromSeed(seed)
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

			mnemonic, err := crypto.SeedToMnemonic(seed)
			if err != nil {
				return err
			}

			fp := crypto.Fingerprint(kp.SignPub)
			fmt.Println("\n=== DEVICE KEY CREATED ===")
			fmt.Printf("Fingerprint: %s\n", fp)
			fmt.Println("\n── RECOVERY PHRASE ──")
			fmt.Println("Write down these 24 words. Store them offline.")
			fmt.Println("Anyone with these words can recover this device's keys (without your passphrase).")
			fmt.Println()
			fmt.Println(mnemonic)
			fmt.Println()
			fmt.Println("Next: `devsync config set server_url <url>` then `devsync register`.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing device key")
	return cmd
}
