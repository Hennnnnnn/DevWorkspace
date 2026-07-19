package actions

import (
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// Invite adds a user directly to a team (admin).
func Invite(user, team string) error {
	cl, _, err := AuthedClient()
	if err != nil {
		return err
	}
	return cl.Post("/admin/invite", protocol.InviteRequest{Username: user, TeamName: team}, nil)
}

// DeleteTeam permanently deletes a team and all its data (admin).
func DeleteTeam(name string) error {
	cl, _, err := AuthedClient()
	if err != nil {
		return err
	}
	return cl.Post("/admin/delete-team", protocol.CreateTeamRequest{Name: name}, nil)
}

// CreateTeam creates a new team (admin).
func CreateTeam(name string) (*protocol.Team, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var t protocol.Team
	if err := cl.Post("/admin/create-team", protocol.CreateTeamRequest{Name: name}, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Join requests to join an existing team (requires admin approval).
func Join(team string) error {
	cl, _, err := AuthedClient()
	if err != nil {
		return err
	}
	return cl.Post("/teams/join", protocol.CreateTeamRequest{Name: team}, nil)
}

// ListTeams returns the teams the caller belongs to.
func ListTeams() ([]protocol.Team, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var out protocol.TeamList
	if err := cl.Get("/teams", nil, &out); err != nil {
		return nil, err
	}
	return out.Teams, nil
}

// ListMembers returns a team's members (optionally only pending ones).
func ListMembers(team string, pending bool) ([]protocol.Member, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	q := urlValues("team", team)
	if pending {
		q.Set("pending", "true")
	}
	var out protocol.MemberList
	if err := cl.Get("/members", q, &out); err != nil {
		return nil, err
	}
	return out.Members, nil
}
