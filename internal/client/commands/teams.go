package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

func newCreateTeamCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create-team <name>",
		Short: "Create a team (admin)",
		Long:  "Create a new team.\n\nArguments:\n  <name>  Team name",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			var t protocol.Team
			if err := cl.Post("/admin/create-team", protocol.CreateTeamRequest{Name: args[0]}, &t); err != nil {
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			if err := cl.Post("/teams/join", protocol.CreateTeamRequest{Name: args[0]}, nil); err != nil {
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			var out protocol.TeamList
			if err := cl.Get("/teams", nil, &out); err != nil {
				return err
			}
			if len(out.Teams) == 0 {
				fmt.Println("(no teams)")
				return nil
			}
			for _, t := range out.Teams {
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			q := urlValues("team", team)
			if pending {
				q.Set("pending", "true")
			}
			var out protocol.MemberList
			if err := cl.Get("/members", q, &out); err != nil {
				return err
			}
			if len(out.Members) == 0 {
				fmt.Println("(no members)")
				return nil
			}
			for _, m := range out.Members {
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
