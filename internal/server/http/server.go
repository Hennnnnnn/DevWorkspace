package http

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/devsync/devsync/internal/protocol"
	"github.com/devsync/devsync/internal/server/store"
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
