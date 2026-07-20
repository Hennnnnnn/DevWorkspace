package http

import (
	"encoding/json"
	"net/http"

	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateTeamRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	t, err := s.store.CreateTeam(r.Context(), req.Name)
	if err != nil {
		writeErr(w, http.StatusConflict, "create team: "+err.Error())
		return
	}
	// Creator becomes active member.
	_ = s.store.AddTeamMember(r.Context(), t.ID, userOf(r).ID, "active")
	_ = s.store.Log(r.Context(), userOf(r).ID, deviceOf(r).ID, "", "create_team", req.Name)
	writeJSON(w, http.StatusOK, protocol.Team{ID: t.ID, Name: t.Name})
}

func (s *Server) handleTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.store.ListTeamsForUser(r.Context(), userOf(r).ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list teams")
		return
	}
	out := protocol.TeamList{}
	for _, t := range teams {
		out.Teams = append(out.Teams, protocol.Team{ID: t.ID, Name: t.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateTeamRequest // reuse {name}
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "team name required")
		return
	}
	t, err := s.store.GetTeamByName(r.Context(), req.Name)
	if err != nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
		if err := s.store.AddTeamMember(r.Context(), t.ID, userOf(r).ID, "pending"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to join team")
		return
	}
	_ = s.store.Log(r.Context(), userOf(r).ID, deviceOf(r).ID, "", "join_team", req.Name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
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
			Username: m.Username, Status: m.Status, Fingerprint: m.Fingerprint,
			DeviceID: m.DeviceID, BoxPubKey: m.BoxPubKey})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteTeam(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateTeamRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "team name required")
		return
	}
	ctx := r.Context()
	if _, err := s.store.GetTeamByName(ctx, req.Name); err != nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
	if err := s.store.DeleteTeam(ctx, req.Name); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to delete team")
		return
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, "", "delete_team", req.Name)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) handleInvite(w http.ResponseWriter, r *http.Request) {
	var req protocol.InviteRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Username == "" || req.TeamName == "" {
		writeErr(w, http.StatusBadRequest, "username and team_name required")
		return
	}
	ctx := r.Context()
	u, err := s.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	t, err := s.store.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
	if err := s.store.AddTeamMember(ctx, t.ID, u.ID, "active"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to add team member")
		return
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, "", "invite", req.Username+" to "+req.TeamName)
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
	if err := s.store.SetUserAdmin(ctx, u.ID, true); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to set admin")
		return
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, "", "set_admin", req.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "admin"})
}

// handleApprove activates a pending user + their pending device, verifying the
// admin-supplied fingerprint matches. Admin also submits sealed vault key shares
// for the newly trusted device.
func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	var req protocol.ApproveRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Username == "" || req.Fingerprint == "" {
		writeErr(w, http.StatusBadRequest, "username + fingerprint required")
		return
	}
	ctx := r.Context()
	device, user, err := s.store.GetDeviceByFingerprint(ctx, req.Fingerprint)
	if err != nil {
		writeErr(w, http.StatusNotFound, "device fingerprint not found")
		return
	}
	if user.Username != req.Username {
		writeErr(w, http.StatusBadRequest, "fingerprint does not match user")
		return
	}
	if err := s.store.SetUserStatus(ctx, user.ID, "active"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to activate user")
		return
	}
	if err := s.store.SetDeviceStatus(ctx, device.ID, "active"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to activate device")
		return
	}
	// Activate any pending team memberships for this user.
	_ = s.store.ActivatePendingMemberships(ctx, user.ID)

	if len(req.Shares) > 0 {
		shares := make([]store.KeyShare, 0, len(req.Shares))
		for _, sh := range req.Shares {
			shares = append(shares, store.KeyShare{
				VaultID: sh.VaultID, DeviceID: sh.DeviceID,
				KeyVersion: sh.KeyVersion, EncryptedKey: sh.EncryptedKey})
		}
		if err := s.store.AddKeyShares(ctx, shares); err != nil {
			writeErr(w, http.StatusInternalServerError, "failed to add vault key shares")
			return
		}
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, "", "approve", req.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "active"})
}
