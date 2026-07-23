package http

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateTeamRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Team == "" {
		writeErr(w, http.StatusBadRequest, "team required")
		return
	}
	t, err := s.store.CreateTeam(r.Context(), req.Team)
	if err != nil {
		writeErr(w, http.StatusConflict, "create team: "+err.Error())
		return
	}
	// Creator becomes active team admin.
	_ = s.store.AddTeamMember(r.Context(), t.ID, userOf(r).ID, "active", "admin")
	_ = s.store.Log(r.Context(), userOf(r).ID, deviceOf(r).ID, "", "create_team", req.Team)
	writeJSON(w, http.StatusOK, protocol.Team{ID: t.ID, Name: t.Name, Creator: userOf(r).Username})
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	var teams []store.Team
	var err error
	switch {
	case r.URL.Query().Get("all") == "true":
		teams, err = s.store.ListAllTeams(r.Context())
	case r.URL.Query().Get("pending") == "true":
		teams, err = s.store.ListTeamsForUser(r.Context(), userOf(r).ID, "pending")
	default:
		teams, err = s.store.ListTeamsForUser(r.Context(), userOf(r).ID, "active")
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list teams")
		return
	}
	out := protocol.TeamList{}
	for _, t := range teams {
		out.Teams = append(out.Teams, protocol.Team{ID: t.ID, Name: t.Name, Creator: t.CreatedBy})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMembers(w http.ResponseWriter, r *http.Request) {
	team := r.URL.Query().Get("team")
	pendingOnly := r.URL.Query().Get("pending") == "true"
	t, err := s.store.GetTeamByName(r.Context(), team)
	if err != nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
	members, err := s.store.ListMembers(r.Context(), t.ID, pendingOnly)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list team members")
		return
	}
	out := protocol.MemberList{}
	for _, m := range members {
		out.Members = append(out.Members, protocol.Member{
			Username: m.Username, Status: m.Status, Role: m.Role,
			Fingerprint: m.Fingerprint, DeviceID: m.DeviceID, BoxPubKey: m.BoxPubKey})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateTeamRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Team == "" {
		writeErr(w, http.StatusBadRequest, "team required")
		return
	}
	ctx := r.Context()
	if _, err := s.store.GetTeamByName(ctx, req.Team); err != nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
	if err := s.store.DeleteTeam(ctx, req.Team); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete team")
		return
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, "", "delete_team", req.Team)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	var req protocol.InviteRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Username == "" || req.Team == "" {
		writeErr(w, http.StatusBadRequest, "username and team required")
		return
	}
	ctx := r.Context()
	t, err := s.store.GetTeamByName(ctx, req.Team)
	if err != nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
	token := generateToken()
	expiresAt := time.Now().Add(24 * time.Hour)
	if err := s.store.CreateInviteToken(ctx, token, t.ID, req.Username, userOf(r).ID, expiresAt); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create invite token")
		return
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, "", "invite", req.Username+" to "+req.Team)
	writeJSON(w, http.StatusOK, protocol.InviteTokenResponse{
		Token:     token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}

func (s *Server) handleClaimInvite(w http.ResponseWriter, r *http.Request) {
	var req protocol.ClaimInviteRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Token == "" {
		writeErr(w, http.StatusBadRequest, "token required")
		return
	}
	ctx := r.Context()
	u := userOf(r)
	d := deviceOf(r)
	if err := s.store.ClaimInviteToken(ctx, req.Token, u.Username, u.ID, d.ID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "token not found")
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.Log(ctx, u.ID, d.ID, "", "claim_invite", req.Token)
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

func (s *Server) handleSetAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Username == "" {
		writeErr(w, http.StatusBadRequest, "username required")
		return
	}
	ctx := r.Context()
	u, err := s.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	// Middleware already verified caller is team_admin. The team is on the body.
	var body struct {
		Team string `json:"team"`
	}
	json.Unmarshal(bodyOf(r), &body)
	t, _ := s.store.GetTeamByName(ctx, body.Team)
	if t == nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
	// Check target is an active member of the team.
	ok, err := s.store.IsTeamAdmin(ctx, t.ID, u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "check target membership failed")
		return
	}
	if ok {
		writeErr(w, http.StatusBadRequest, "user is already team admin")
		return
	}
	members, _ := s.store.ListMembers(ctx, t.ID, false)
	isMember := false
	for _, m := range members {
		if m.Username == req.Username && m.Status == "active" {
			isMember = true
			break
		}
	}
	if !isMember {
		writeErr(w, http.StatusBadRequest, "user is not an active member of this team")
		return
	}
	if err := s.store.SetTeamMemberRole(ctx, t.ID, u.ID, "admin"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to set team admin")
		return
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, "", "set_team_admin", req.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "team_admin"})
}

func generateToken() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
}
