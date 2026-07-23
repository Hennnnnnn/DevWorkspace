package http

import (
	"encoding/base64"
	"encoding/json"
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

	// Authenticated (signature required; pending devices allowed for whoami).
	mux.HandleFunc("GET /whoami", s.authed(s.handleWhoAmI))

	// Authenticated only — general feature access (RFC §1: device activation no
	// longer a prerequisite). Pending devices can still reach link/invite flows.
	mux.HandleFunc("GET /teams", s.authed(s.handleTeams))
	mux.HandleFunc("POST /teams/claim", s.authed(s.handleClaimInvite))
	mux.HandleFunc("GET /teams/members", s.authed(s.handleMembers))
	mux.HandleFunc("GET /vaults", s.authed(s.handleVaults))
	mux.HandleFunc("GET /files", s.authed(s.handleListFiles))
	mux.HandleFunc("GET /files/history", s.authed(s.handleHistory))
	mux.HandleFunc("GET /devices", s.authed(s.handleListDevices))
	mux.HandleFunc("GET /users/devices", s.authed(s.handleUserDevices))
	mux.HandleFunc("POST /devices/link", s.authed(s.handleLinkDevice))
	mux.HandleFunc("GET /audit", s.authed(s.handleAudit))

	// Vault-unlock sensitive (RFC §3). Server can't observe client agent unlock
	// state, so equivalent mechanism = trusted-device gate: only active devices
	// receive sealed vault keys from an admin. These expose sealed keys/ciphertext.
	mux.HandleFunc("GET /vaults/keyshares", s.activeAuthed(s.handleKeyShares))
	mux.HandleFunc("POST /files/push", s.activeAuthed(s.handlePush))
	mux.HandleFunc("GET /files/pull", s.activeAuthed(s.handlePull))

	// Team admin only (admin must be a trusted/active device).
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
