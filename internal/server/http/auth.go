package http

import (
	"context"
	"crypto/ed25519"
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

// activeAuthed is authed + requires the device and user to be active.
func (s *Server) activeAuthed(next http.HandlerFunc) http.HandlerFunc {
	return s.authed(func(w http.ResponseWriter, r *http.Request) {
		d := deviceOf(r)
		u := userOf(r)
		if d.Status != "active" {
			writeErr(w, http.StatusForbidden, "device not active")
			return
		}
		if u.Status != "active" {
			writeErr(w, http.StatusForbidden, "user not active")
			return
		}
		next(w, r)
	})
}

// adminAuthed is activeAuthed + requires admin.
func (s *Server) adminAuthed(next http.HandlerFunc) http.HandlerFunc {
	return s.activeAuthed(func(w http.ResponseWriter, r *http.Request) {
		if !userOf(r).IsAdmin {
			writeErr(w, http.StatusForbidden, "admin only")
			return
		}
		next(w, r)
	})
}

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
