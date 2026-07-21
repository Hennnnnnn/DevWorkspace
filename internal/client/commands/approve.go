package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

func newApproveCmd() *cobra.Command {
	var fingerprint string
	cmd := &cobra.Command{
		Use:   "approve <user> --fingerprint <fp>",
		Short: "Approve a pending user after verifying their fingerprint out-of-band (admin)",
		Long:  "Approve a pending user and re-share vault keys to their devices.\n\nArguments:\n  <user>  Username to approve",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			res, err := actions.Approve(args[0], fingerprint, nil)
			if err != nil {
				return err
			}
			fmt.Printf("approved %s\n", args[0])
			if res.ShareNote != "" {
				fmt.Printf("note: %s\n", res.ShareNote)
				return nil
			}
			if res.VaultsShared > 0 {
				fmt.Printf("re-shared %d vault key(s) to %s\n", res.VaultsShared, args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "device fingerprint (verify out-of-band)")
	return cmd
}
