package http

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

// handleRegister creates a user+device (first device) or adds a device to an
// existing user. First device is pending (admin approves). A second device that
// carries a valid link signature from an existing active device is auto-active.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req protocol.RegisterRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if req.Username == "" || len(req.SignPubKey) != ed25519.PublicKeySize || len(req.BoxPubKey) != 32 {
		writeErr(w, http.StatusBadRequest, "missing/invalid fields")
		return
	}
	if len(req.Username) > 255 {
		writeErr(w, http.StatusBadRequest, "username too long (max 255)")
		return
	}
	if len(req.DeviceName) > 255 {
		writeErr(w, http.StatusBadRequest, "device name too long (max 255)")
		return
	}
	// Verify the fingerprint matches the submitted signing key (no spoofing).
	if crypto.Fingerprint(ed25519.PublicKey(req.SignPubKey)) != req.Fingerprint {
		writeErr(w, http.StatusBadRequest, "fingerprint mismatch")
		return
	}

	ctx := r.Context()
	existing, err := s.store.GetUserByUsername(ctx, req.Username)

	// New user -> new pending device.
	if errors.Is(err, store.ErrNotFound) {
		uid, did, err := s.store.CreateUserWithDevice(ctx,
			store.User{Username: req.Username, Status: "pending"},
			store.Device{Name: req.DeviceName, SignPubKey: req.SignPubKey, BoxPubKey: req.BoxPubKey,
				Fingerprint: req.Fingerprint, Status: "pending"})
		if err != nil {
			writeErr(w, http.StatusConflict, "register failed: "+err.Error())
			return
		}
		_ = s.store.Log(ctx, uid, did, "", "register", req.Username)
		writeJSON(w, http.StatusOK, protocol.RegisterResponse{UserID: uid, DeviceID: did, Status: "pending"})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to look up user")
		return
	}

	// Existing user adding a device. Requires a valid link signature from one of
	// their active devices over the new fingerprint.
	status := "pending"
	if len(req.LinkSignature) > 0 && req.LinkDeviceFingerprint != "" {
		signer, signerUser, err := s.store.GetDeviceByFingerprint(ctx, req.LinkDeviceFingerprint)
		if err != nil || signerUser.ID != existing.ID || signer.Status != "active" {
			writeErr(w, http.StatusForbidden, "invalid link device")
			return
		}
		if ed25519.Verify(ed25519.PublicKey(signer.SignPubKey), []byte(req.Fingerprint), req.LinkSignature) {
			status = "active"
		} else {
			writeErr(w, http.StatusForbidden, "invalid link signature")
			return
		}
	}
	did, err := s.store.AddDevice(ctx, store.Device{
		UserID: existing.ID, Name: req.DeviceName, SignPubKey: req.SignPubKey,
		BoxPubKey: req.BoxPubKey, Fingerprint: req.Fingerprint, Status: status})
	if err != nil {
		writeErr(w, http.StatusConflict, "add device failed: "+err.Error())
		return
	}
	_ = s.store.Log(ctx, existing.ID, did, "", "device_add", req.DeviceName)
	writeJSON(w, http.StatusOK, protocol.RegisterResponse{UserID: existing.ID, DeviceID: did, Status: status})
}

func (s *Server) handleWhoAmI(w http.ResponseWriter, r *http.Request) {
	u := userOf(r)
	d := deviceOf(r)
	writeJSON(w, http.StatusOK, protocol.WhoAmIResponse{
		Username: u.Username, Status: u.Status, IsAdmin: u.IsAdmin,
		Device: protocol.Device{ID: d.ID, Name: d.Name, Fingerprint: d.Fingerprint, Status: d.Status},
	})
}

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devs, err := s.store.ListDevices(r.Context(), userOf(r).ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	out := protocol.DeviceList{}
	for _, d := range devs {
		out.Devices = append(out.Devices, protocol.Device{
			ID: d.ID, Name: d.Name, Fingerprint: d.Fingerprint, Status: d.Status, BoxPubKey: d.BoxPubKey})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleUserDevices returns another user's active devices + box keys, so an
// admin/member can seal vault keys to them (grant/approve). Box keys are public.
func (s *Server) handleUserDevices(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	u, err := s.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	devs, err := s.store.ListActiveDevices(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	out := protocol.DeviceList{}
	for _, d := range devs {
		out.Devices = append(out.Devices, protocol.Device{
			ID: d.ID, Name: d.Name, Fingerprint: d.Fingerprint, Status: d.Status, BoxPubKey: d.BoxPubKey})
	}
	writeJSON(w, http.StatusOK, out)
}

// handleLinkDevice lets an active device revoke a device it owns.
func (s *Server) handleLinkDevice(w http.ResponseWriter, r *http.Request) {
	var req protocol.RevokeRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if req.DeviceID == "" {
		writeErr(w, http.StatusBadRequest, "device_id required")
		return
	}
	// Ensure the target device belongs to the caller.
	devs, err := s.store.ListDevices(r.Context(), userOf(r).ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to look up devices")
		return
	}
	owns := false
	for _, d := range devs {
		if d.ID == req.DeviceID {
			owns = true
		}
	}
	if !owns {
		writeErr(w, http.StatusForbidden, "not your device")
		return
	}
	if err := s.store.SetDeviceStatus(r.Context(), req.DeviceID, "revoked"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to revoke device")
		return
	}
	_ = s.store.Log(r.Context(), userOf(r).ID, deviceOf(r).ID, "", "device_revoke", req.DeviceID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}
