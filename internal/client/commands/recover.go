package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
)

func newRecoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recover <mnemonic>",
		Short: "Recover device keys from a 24-word recovery phrase",
		Long: `Restore device keys from the recovery phrase printed during init.
The same key material is derived deterministically — your fingerprint
and all encrypted vault access are preserved.

After recovery, run 'devsync register' to link this device to your account.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if keystore.Exists() {
				return fmt.Errorf("device key already exists; remove it first or use 'init --force'")
			}

			seed, err := crypto.MnemonicToSeed(args[0])
			if err != nil {
				return fmt.Errorf("invalid recovery phrase: %w", err)
			}

			kp, err := crypto.DeriveKeyPairFromSeed(seed)
			if err != nil {
				return fmt.Errorf("key derivation: %w", err)
			}

			fp := crypto.Fingerprint(kp.SignPub)
			fmt.Printf("Recovered fingerprint: %s\n", fp)

			pass, err := promptPassphrase("Choose a NEW passphrase for the recovered key: ")
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

			ek, err := crypto.EncryptPrivateKey(kp, pass)
			if err != nil {
				return err
			}
			if err := keystore.Save(ek); err != nil {
				return err
			}

			fmt.Println("\nDevice key restored.")
			fmt.Println("Next: devsync register   (your fingerprint matches the original device)")
			return nil
		},
	}
}
