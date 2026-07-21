package commands

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

func newUnlockCmd() *cobra.Command {
	var ttl time.Duration
	cmd := &cobra.Command{
		Use:   "unlock",
		Short: "Unlock the device key into the agent for a period",
		RunE: func(_ *cobra.Command, _ []string) error {
			pass, err := promptPassphrase("Passphrase to unlock this device: ")
			if err != nil {
				return err
			}
			if err := actions.Unlock(pass, ttl); err != nil {
				return err
			}
			fmt.Printf("unlocked for %s\n", ttl)
			return nil
		},
	}
	cmd.Flags().DurationVar(&ttl, "timeout", 8*time.Hour, "how long the key stays unlocked")

	lock := &cobra.Command{
		Use:   "lock",
		Short: "Forget the unlocked key immediately",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := actions.Lock(); err != nil {
				return err
			}
			fmt.Println("locked")
			return nil
		},
	}
	cmd.AddCommand(lock)
	return cmd
}
