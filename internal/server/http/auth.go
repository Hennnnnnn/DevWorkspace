package http

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxDevice
	ctxBody
)

// authed wraps a handler with signature verification + anti-replay.
// On success the request's user, device, and raw body are stored in context.
func (s *Server) authed(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20)) // 2 MiB ceiling
		if err != nil {
			writeErr(w, http.StatusBadRequest, "read body")
			return
		}

		fp := r.Header.Get(protocol.HeaderDevice)
		tsStr := r.Header.Get(protocol.HeaderTimestamp)
		sig := r.Header.Get(protocol.HeaderSignature)
		if fp == "" || tsStr == "" || sig == "" {
			writeErr(w, http.StatusUnauthorized, "missing auth headers")
			return
		}

		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "bad timestamp")
			return
		}
		if skew := time.Now().Unix() - ts; skew > protocol.MaxSkewSeconds || skew < -protocol.MaxSkewSeconds {
			writeErr(w, http.StatusUnauthorized, "timestamp out of range (replay?)")
			return
		}

		device, user, err := s.store.GetDeviceByFingerprint(r.Context(), fp)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "unknown device")
			return
		}

		msg := protocol.SigningString(r.Method, r.URL.RequestURI(), protocol.BodyHash(body), ts)
		if !ed25519.Verify(ed25519.PublicKey(device.SignPubKey), msg, decodeSig(sig)) {
			writeErr(w, http.StatusUnauthorized, "bad signature")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxDevice, device)
		ctx = context.WithValue(ctx, ctxBody, body)
		next(w, r.WithContext(ctx))
	}
}

// activeAuthed is authed + requires the device to be active. Acts as the
// server-side vault-unlock gate (RFC §3, "or equivalent mechanism"): the server
// can't observe the client agent's unlock state, but only an active/trusted
// device receives sealed vault keys from an admin, so sensitive endpoints
// (keyshares, file I/O) require it. General feature routes use plain authed
// (RFC §1) — device activation is no longer a prerequisite for using the app.
func (s *Server) activeAuthed(next http.HandlerFunc) http.HandlerFunc {
	return s.authed(func(w http.ResponseWriter, r *http.Request) {
		if deviceOf(r).Status != "active" {
			writeErr(w, http.StatusForbidden, "device not active")
			return
		}
		next(w, r)
	})
}

// teamAdminAuthed is activeAuthed + requires team admin role. The body must
// contain a "team" field.
func (s *Server) teamAdminAuthed(next http.HandlerFunc) http.HandlerFunc {
	return s.activeAuthed(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Team string `json:"team"`
		}
		if err := json.Unmarshal(bodyOf(r), &body); err != nil || body.Team == "" {
			writeErr(w, http.StatusBadRequest, "team required")
			return
		}
		t, err := s.store.GetTeamByName(r.Context(), body.Team)
		if err != nil {
			writeErr(w, http.StatusNotFound, "team not found")
			return
		}
		ok, err := s.store.IsTeamAdmin(r.Context(), t.ID, userOf(r).ID)
		if err != nil || !ok {
			writeErr(w, http.StatusForbidden, "team_admin only")
			return
		}
		// Store resolved team in context for handlers to reuse.
		ctx := context.WithValue(r.Context(), ctxTeam, t)
		next(w, r.WithContext(ctx))
	})
}

type ctxTeamKey int

const ctxTeam ctxTeamKey = iota + 3 // avoid collision with ctxKey values

func teamOf(r *http.Request) *store.Team { return r.Context().Value(ctxTeam).(*store.Team) }

func userOf(r *http.Request) *store.User     { return r.Context().Value(ctxUser).(*store.User) }
func deviceOf(r *http.Request) *store.Device { return r.Context().Value(ctxDevice).(*store.Device) }
func bodyOf(r *http.Request) []byte {
	b, _ := r.Context().Value(ctxBody).([]byte)
	return b
}

func decodeSig(s string) []byte {
	// base64 std; invalid -> nil -> Verify fails.
	out, err := base64Decode(s)
	if err != nil {
		return nil
	}
	return out
}
