package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

func newInviteCmd() *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "invite <user> --team <team>",
		Short: "Invite a user to a team (admin)",
		Long:  "Add a user to a team directly. They don't need to request join first.\n\nArguments:\n  <user>  Username to invite",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := actions.Invite(args[0], team); err != nil {
				return err
			}
			fmt.Printf("invited %s to team %q\n", args[0], team)
			return nil
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "team name")
	_ = cmd.MarkFlagRequired("team")
	return cmd
}

func newDeleteTeamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete-team <name>",
		Short: "Delete a team and all its data (admin)",
		Long:  "Permanently delete a team, its vaults, files, and memberships.\n\nArguments:\n  <name>  Team name to delete",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := actions.DeleteTeam(args[0]); err != nil {
				return err
			}
			fmt.Printf("team %q deleted\n", args[0])
			return nil
		},
	}
}

func newCreateTeamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create-team <name>",
		Short: "Create a team (admin)",
		Long:  "Create a new team.\n\nArguments:\n  <name>  Team name",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			t, err := actions.CreateTeam(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("team %q created\n", t.Name)
			return nil
		},
	}
}

func newJoinCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "join <team>",
		Short: "Request to join a team",
		Long:  "Request to join an existing team (requires admin approval).\n\nArguments:\n  <team>  Team name to join",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := actions.Join(args[0]); err != nil {
				return err
			}
			fmt.Printf("join request sent for %q — pending approval\n", args[0])
			return nil
		},
	}
}

func newTeamsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "teams",
		Short: "List teams you belong to",
		RunE: func(_ *cobra.Command, _ []string) error {
			teams, err := actions.ListTeams()
			if err != nil {
				return err
			}
			if len(teams) == 0 {
				fmt.Println("(no teams)")
				return nil
			}
			for _, t := range teams {
				fmt.Println(t.Name)
			}
			return nil
		},
	}
}

func newMembersCmd() *cobra.Command {
	var team string
	var pending bool
	cmd := &cobra.Command{
		Use:   "members",
		Short: "List team members",
		RunE: func(_ *cobra.Command, _ []string) error {
			members, err := actions.ListMembers(team, pending)
			if err != nil {
				return err
			}
			if len(members) == 0 {
				fmt.Println("(no members)")
				return nil
			}
			for _, m := range members {
				fmt.Printf("%-20s %-8s %s\n", m.Username, m.Status, m.Fingerprint)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "team name")
	cmd.Flags().BoolVar(&pending, "pending", false, "only pending members")
	_ = cmd.MarkFlagRequired("team")
	return cmd
}
