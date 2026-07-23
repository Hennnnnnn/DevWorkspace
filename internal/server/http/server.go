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
	mux.HandleFunc("POST /register", rateLimit(s.handleRegister))

	// Bootstrap — one-shot: activates the first user. Only works when
	// zero active users exist. Safe to call right after /register.
	mux.HandleFunc("POST /bootstrap", rateLimit(s.handleBootstrap))

	// Authenticated (signature required; pending devices allowed for whoami).
	mux.HandleFunc("GET /whoami", s.authed(s.handleWhoAmI))

	// Active user required.
	mux.HandleFunc("GET /teams", s.activeAuthed(s.handleTeams))
	mux.HandleFunc("POST /teams/claim", s.authed(s.handleClaimInvite))
	mux.HandleFunc("GET /teams/members", s.activeAuthed(s.handleMembers))
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

	// Team admin only.
	mux.HandleFunc("POST /teams/create", s.activeAuthed(s.handleCreateTeam))
	mux.HandleFunc("POST /teams/delete", s.teamAdminAuthed(s.handleDeleteTeam))
	mux.HandleFunc("POST /teams/vaults/create", s.teamAdminAuthed(s.handleCreateVault))
	mux.HandleFunc("POST /teams/vaults/grant", s.teamAdminAuthed(s.handleGrant))
	mux.HandleFunc("POST /teams/vaults/revoke", s.teamAdminAuthed(s.handleRevoke))
	mux.HandleFunc("POST /teams/invite", s.teamAdminAuthed(s.handleInvite))
	mux.HandleFunc("POST /teams/set-admin", s.teamAdminAuthed(s.handleSetAdmin))

	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
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

// handleBootstrap activates the first registered user. Only works when zero
// active users exist (one-shot). The user must have already called /register.
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
	if len(req.Username) > 255 {
		writeErr(w, http.StatusBadRequest, "username too long (max 255)")
		return
	}
	if len(req.Fingerprint) > 200 {
		writeErr(w, http.StatusBadRequest, "fingerprint too long")
		return
	}

	ctx := r.Context()
	n, err := s.store.CountActiveUsers(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to check existing active users")
		return
	}
	if n > 0 {
		writeErr(w, http.StatusForbidden, "active user already exists")
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

	writeJSON(w, http.StatusOK, map[string]string{"status": "active", "username": user.Username})
}
