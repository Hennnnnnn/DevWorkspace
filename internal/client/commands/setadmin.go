package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

func newSetAdminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-admin <user>",
		Short: "Promote a user to admin (admin only)",
		Long:  "Grant admin privileges to an existing active user.\n\nArguments:\n  <user>  Username to promote",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := actions.AuthedClient()
			if err != nil {
				return err
			}
			if err := cl.Post("/admin/set-admin", map[string]string{"username": args[0]}, nil); err != nil {
				return err
			}
			fmt.Printf("%s is now admin\n", args[0])
			return nil
		},
	}
}
