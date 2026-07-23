package actions

import (
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// Invite creates an invite token for a user to join a team (team admin).
func Invite(user, team string) (*protocol.InviteTokenResponse, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var resp protocol.InviteTokenResponse
	if err := cl.Post("/teams/invite", protocol.InviteRequest{Username: user, Team: team}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteTeam permanently deletes a team and all its data (team admin).
func DeleteTeam(name string) error {
	cl, _, err := AuthedClient()
	if err != nil {
		return err
	}
	return cl.Post("/teams/delete", protocol.CreateTeamRequest{Team: name}, nil)
}

// CreateTeam creates a new team.
func CreateTeam(name string) (*protocol.Team, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var t protocol.Team
	if err := cl.Post("/teams/create", protocol.CreateTeamRequest{Team: name}, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ClaimInvite claims a team invite token. Activates user+device and adds to team.
// ponytail: room-based ownership — joining is invite-only (owner issues token).
func ClaimInvite(token string) error {
	cl, _, err := AuthedClient()
	if err != nil {
		return err
	}
	return cl.Post("/teams/claim", protocol.ClaimInviteRequest{Token: token}, nil)
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

// ListAllTeams returns every team in the system.
func ListAllTeams() ([]protocol.Team, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	q := urlValues()
	q.Set("all", "true")
	var out protocol.TeamList
	if err := cl.Get("/teams", q, &out); err != nil {
		return nil, err
	}
	return out.Teams, nil
}

// ListPendingTeams returns teams the caller has a pending join request for.
func ListPendingTeams() ([]protocol.Team, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	q := urlValues()
	q.Set("pending", "true")
	var out protocol.TeamList
	if err := cl.Get("/teams", q, &out); err != nil {
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
	if err := cl.Get("/teams/members", q, &out); err != nil {
		return nil, err
	}
	return out.Members, nil
}
