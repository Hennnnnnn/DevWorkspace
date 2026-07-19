package http

import (
	"encoding/json"
	"net/http"

	"github.com/devsync/devsync/internal/protocol"
	"github.com/devsync/devsync/internal/server/store"
)

func toStoreShares(in []protocol.VaultKeyShare, vaultID string) []store.KeyShare {
	out := make([]store.KeyShare, 0, len(in))
	for _, sh := range in {
		vid := sh.VaultID
		if vid == "" {
			vid = vaultID
		}
		out = append(out, store.KeyShare{
			VaultID: vid, DeviceID: sh.DeviceID,
			KeyVersion: sh.KeyVersion, EncryptedKey: sh.EncryptedKey})
	}
	return out
}

func (s *Server) handleCreateVault(w http.ResponseWriter, r *http.Request) {
	var req protocol.CreateVaultRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Team == "" || req.Name == "" {
		writeErr(w, http.StatusBadRequest, "team + name required")
		return
	}
	ctx := r.Context()
	t, err := s.store.GetTeamByName(ctx, req.Team)
	if err != nil {
		writeErr(w, http.StatusNotFound, "team not found")
		return
	}
	v, err := s.store.CreateVault(ctx, t.ID, req.Name, toStoreShares(req.Shares, ""))
	if err != nil {
		writeErr(w, http.StatusConflict, "create vault: "+err.Error())
		return
	}
	// Grant creator access so their shares resolve.
	_ = s.store.Grant(ctx, v.ID, userOf(r).ID, nil)
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, v.ID, "create_vault", req.Name)
	writeJSON(w, http.StatusOK, protocol.Vault{ID: v.ID, Team: req.Team, Name: req.Name})
}

func (s *Server) handleVaults(w http.ResponseWriter, r *http.Request) {
	vaults, teams, err := s.store.ListVaultsForUser(r.Context(), userOf(r).ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list vaults")
		return
	}
	out := protocol.VaultList{}
	for i, v := range vaults {
		out.Vaults = append(out.Vaults, protocol.Vault{ID: v.ID, Team: teams[i], Name: v.Name})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleKeyShares returns the caller device's sealed vault keys for a vault.
func (s *Server) handleKeyShares(w http.ResponseWriter, r *http.Request) {
	vaultName := r.URL.Query().Get("vault")
	v, err := s.resolveVaultForUser(r, vaultName)
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	shares, err := s.store.GetKeySharesForDevice(r.Context(), v.ID, deviceOf(r).ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "keyshares")
		return
	}
	out := protocol.KeySharesResponse{}
	for _, sh := range shares {
		out.Shares = append(out.Shares, protocol.VaultKeyShare{
			VaultID: sh.VaultID, DeviceID: sh.DeviceID,
			KeyVersion: sh.KeyVersion, EncryptedKey: sh.EncryptedKey})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGrant(w http.ResponseWriter, r *http.Request) {
	var req protocol.GrantRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Username == "" || req.Vault == "" {
		writeErr(w, http.StatusBadRequest, "username + vault required")
		return
	}
	ctx := r.Context()
	// Admin must have access to the vault (holds the key) — enforced by requiring
	// the admin submit sealed shares (they can only seal if they hold the key).
	v, err := s.store.GetVaultByName(ctx, req.Vault)
	if err != nil {
		writeErr(w, http.StatusNotFound, "vault not found")
		return
	}
	ok, _ := s.store.HasGrant(ctx, v.ID, userOf(r).ID)
	if !ok {
		writeErr(w, http.StatusForbidden, "admin lacks vault access")
		return
	}
	grantee, err := s.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if err := s.store.Grant(ctx, v.ID, grantee.ID, toStoreShares(req.Shares, v.ID)); err != nil {
		writeErr(w, http.StatusInternalServerError, "grant")
		return
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, v.ID, "grant", req.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "granted"})
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	var req protocol.RevokeRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Vault == "" || req.Username == "" {
		writeErr(w, http.StatusBadRequest, "username + vault required")
		return
	}
	ctx := r.Context()
	v, err := s.store.GetVaultByName(ctx, req.Vault)
	if err != nil {
		writeErr(w, http.StatusNotFound, "vault not found")
		return
	}
	target, err := s.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if err := s.store.RevokeGrant(ctx, v.ID, target.ID); err != nil {
		writeErr(w, http.StatusInternalServerError, "revoke")
		return
	}
	// Rotation: new sealed shares to survivors + re-encrypted files under new key version.
	if len(req.Shares) > 0 {
		if err := s.store.AddKeyShares(ctx, toStoreShares(req.Shares, v.ID)); err != nil {
			writeErr(w, http.StatusInternalServerError, "rotate shares")
			return
		}
	}
	for _, f := range req.Files {
		if _, err := s.store.PushFile(ctx, v.ID, f.Path, f.KeyVersion, f.BaseVersion,
			f.Ciphertext, len(f.Ciphertext), deviceOf(r).ID, false); err != nil {
			writeErr(w, http.StatusConflict, "rotate file "+f.Path+": "+err.Error())
			return
		}
	}
	_ = s.store.Log(ctx, userOf(r).ID, deviceOf(r).ID, v.ID, "revoke", req.Username)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// resolveVaultForUser looks up a vault by name and checks the caller has a grant.
func (s *Server) resolveVaultForUser(r *http.Request, vaultName string) (*store.Vault, error) {
	v, err := s.store.GetVaultByName(r.Context(), vaultName)
	if err != nil {
		return nil, errForbidden("vault not found")
	}
	ok, _ := s.store.HasGrant(r.Context(), v.ID, userOf(r).ID)
	if !ok {
		return nil, errForbidden("no access to vault")
	}
	return v, nil
}

type forbiddenErr string

func (e forbiddenErr) Error() string { return string(e) }
func errForbidden(msg string) error  { return forbiddenErr(msg) }
