package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

// Server holds deps and builds the HTTP handler.
type Server struct {
	store *store.Store
}

func New(st *store.Store) *Server {
	return &Server{store: st}
}

// Handler returns the root mux with all routes wired.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	// Open (self-signed by the registering device, not yet trusted).
	mux.HandleFunc("POST /register", s.handleRegister)

	// Bootstrap — one-shot: promotes the first user to admin. Only works when
	// zero admins exist. Safe to call right after /register.
	mux.HandleFunc("POST /admin/bootstrap", s.handleBootstrap)

	// Authenticated (signature required; pending devices allowed for whoami).
	mux.HandleFunc("GET /whoami", s.authed(s.handleWhoAmI))

	// Active user required.
	mux.HandleFunc("GET /teams", s.activeAuthed(s.handleTeams))
	mux.HandleFunc("POST /teams/join", s.activeAuthed(s.handleJoin))
	mux.HandleFunc("GET /members", s.activeAuthed(s.handleMembers))
	mux.HandleFunc("GET /vaults", s.activeAuthed(s.handleVaults))
	mux.HandleFunc("GET /vaults/keyshares", s.activeAuthed(s.handleKeyShares))
	mux.HandleFunc("POST /files/push", s.activeAuthed(s.handlePush))
	mux.HandleFunc("GET /files", s.activeAuthed(s.handleListFiles))
	mux.HandleFunc("GET /files/pull", s.activeAuthed(s.handlePull))
	mux.HandleFunc("GET /files/history", s.activeAuthed(s.handleHistory))
	mux.HandleFunc("GET /devices", s.activeAuthed(s.handleListDevices))
	mux.HandleFunc("GET /users/devices", s.activeAuthed(s.handleUserDevices))
	mux.HandleFunc("POST /devices/link", s.activeAuthed(s.handleLinkDevice))
	mux.HandleFunc("GET /audit", s.activeAuthed(s.handleAudit))

	// Admin only.
	mux.HandleFunc("POST /admin/create-team", s.adminAuthed(s.handleCreateTeam))
	mux.HandleFunc("POST /admin/approve", s.adminAuthed(s.handleApprove))
	mux.HandleFunc("POST /admin/create-vault", s.adminAuthed(s.handleCreateVault))
	mux.HandleFunc("POST /admin/grant", s.adminAuthed(s.handleGrant))
	mux.HandleFunc("POST /admin/revoke", s.adminAuthed(s.handleRevoke))
	mux.HandleFunc("POST /admin/invite", s.adminAuthed(s.handleInvite))

	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_down"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, protocol.ErrorResponse{Error: msg})
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// handleBootstrap promotes a registered user to admin. Only works when zero
// admins exist (one-shot). The user must have already called /register.
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json")
		return
	}
	if req.Username == "" || req.Fingerprint == "" {
		writeErr(w, http.StatusBadRequest, "username and fingerprint required")
		return
	}

	ctx := r.Context()
	n, err := s.store.CountAdmins(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to check existing admins")
		return
	}
	if n > 0 {
		writeErr(w, http.StatusForbidden, "admin already exists")
		return
	}

	dev, user, err := s.store.GetDeviceByFingerprint(ctx, req.Fingerprint)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "device not found — register first")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to look up device")
		return
	}
	if user.Username != req.Username {
		writeErr(w, http.StatusBadRequest, "fingerprint belongs to a different user")
		return
	}

	if err := s.store.SetUserStatus(ctx, user.ID, "active"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to activate user")
		return
	}
	if err := s.store.SetDeviceStatus(ctx, dev.ID, "active"); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to activate device")
		return
	}
	if err := s.store.SetUserAdmin(ctx, user.ID, true); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to promote to admin")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "admin", "username": user.Username})
}
