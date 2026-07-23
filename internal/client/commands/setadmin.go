package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

func newSetAdminCmd() *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "set-admin <user> --team <team>",
		Short: "Promote a member to team admin (team admin only)",
		Long:  "Grant team admin privileges to an active team member.\n\nArguments:\n  <user>  Username to promote",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := actions.AuthedClient()
			if err != nil {
				return err
			}
			req := map[string]string{"username": args[0], "team": team}
			if err := cl.Post("/teams/set-admin", req, nil); err != nil {
				return err
			}
			fmt.Printf("%s is now team admin in %q\n", args[0], team)
			return nil
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "team name")
	_ = cmd.MarkFlagRequired("team")
	return cmd
}
