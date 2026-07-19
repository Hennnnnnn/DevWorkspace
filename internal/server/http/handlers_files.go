package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

// MaxCiphertext caps a single encrypted blob (~1 MB plaintext + overhead).
const MaxCiphertext = 1 << 20

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	var req protocol.PushRequest
	if err := json.Unmarshal(bodyOf(r), &req); err != nil || req.Vault == "" || req.File.Path == "" {
		writeErr(w, http.StatusBadRequest, "vault + file.path required")
		return
	}
	if len(req.File.Ciphertext) > MaxCiphertext {
		writeErr(w, http.StatusRequestEntityTooLarge, "blob exceeds 1MB limit")
		return
	}
	v, err := s.resolveVaultForUser(r, req.Vault)
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	version, err := s.store.PushFile(r.Context(), v.ID, req.File.Path, req.File.KeyVersion,
		req.File.BaseVersion, req.File.Ciphertext, len(req.File.Ciphertext), deviceOf(r).ID, req.File.Deleted)
	if errors.Is(err, store.ErrVersionConflict) {
		writeErr(w, http.StatusConflict, "stale push — pull first")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "push: "+err.Error())
		return
	}
	action := "push"
	if req.File.Deleted {
		action = "delete"
	}
	_ = s.store.Log(r.Context(), userOf(r).ID, deviceOf(r).ID, v.ID, action, req.File.Path)
	writeJSON(w, http.StatusOK, protocol.PushResponse{Version: version})
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request) {
	v, err := s.resolveVaultForUser(r, r.URL.Query().Get("vault"))
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	files, err := s.store.ListFiles(r.Context(), v.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "list files")
		return
	}
	out := protocol.FileListResponse{}
	for _, f := range files {
		out.Files = append(out.Files, protocol.FileMeta{
			Path: f.Path, LatestVersion: f.LatestVersion, Deleted: f.Deleted})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	v, err := s.resolveVaultForUser(r, r.URL.Query().Get("vault"))
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeErr(w, http.StatusBadRequest, "path required")
		return
	}
	version := 0
	if vs := r.URL.Query().Get("version"); vs != "" {
		version, _ = strconv.Atoi(vs)
	}
	fv, err := s.store.GetFileVersion(r.Context(), v.ID, path, version)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "file not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "pull")
		return
	}
	_ = s.store.Log(r.Context(), userOf(r).ID, deviceOf(r).ID, v.ID, "pull", path)
	writeJSON(w, http.StatusOK, protocol.PullResponse{
		Path: path, Version: fv.Version, KeyVersion: fv.KeyVersion,
		Ciphertext: fv.Ciphertext, Deleted: fv.Deleted})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	v, err := s.resolveVaultForUser(r, r.URL.Query().Get("vault"))
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	path := r.URL.Query().Get("path")
	versions, err := s.store.History(r.Context(), v.ID, path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "history")
		return
	}
	out := protocol.HistoryResponse{}
	for _, fv := range versions {
		out.Entries = append(out.Entries, protocol.HistoryEntry{
			Version: fv.Version, KeyVersion: fv.KeyVersion, SizeBytes: fv.SizeBytes,
			Deleted: fv.Deleted, AuthorDevice: fv.AuthorDevice,
			CreatedAt: fv.CreatedAt.Format("2006-01-02 15:04:05")})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	v, err := s.resolveVaultForUser(r, r.URL.Query().Get("vault"))
	if err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	rows, err := s.store.Audit(r.Context(), v.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "audit")
		return
	}
	out := protocol.AuditResponse{}
	for _, a := range rows {
		out.Entries = append(out.Entries, protocol.AuditEntry{
			Username: a.Username, Device: a.Device, Action: a.Action,
			Target: a.Target, CreatedAt: a.CreatedAt.Format("2006-01-02 15:04:05")})
	}
	writeJSON(w, http.StatusOK, out)
}
