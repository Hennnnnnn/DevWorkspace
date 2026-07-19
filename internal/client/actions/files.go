package actions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

const MaxPlaintext = 1 << 20 // 1 MB

// DoneFunc reports a step's completion message.
type DoneFunc func(msg string)

// StepFunc reports a step's start message and returns a DoneFunc to report
// its completion. CLI callers drive a spinner; TUI callers drive program.Send.
type StepFunc func(msg string) DoneFunc

// Push encrypts and uploads a local file to a vault.
func Push(file, vault string, onStep StepFunc) (version int, err error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return 0, err
	}
	name := filepath.Base(file)

	done := onStep("reading " + name)
	plain, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	if len(plain) > MaxPlaintext {
		return 0, fmt.Errorf("file exceeds 1MB limit (%d bytes)", len(plain))
	}
	done("read " + name)

	done = onStep("fetching vault key")
	vk, keyVersion, err := fetchVaultKey(cl, vault)
	if err != nil {
		return 0, err
	}
	done("got vault key")

	done = onStep(fmt.Sprintf("encrypting %s (%.1f KB)", name, float64(len(plain))/1024))
	ct, err := crypto.EncryptBlob(vk, plain)
	if err != nil {
		return 0, err
	}
	done("encrypted")

	// Determine base version (0 if new) for optimistic lock.
	base, err := currentVersion(cl, vault, name)
	if err != nil {
		return 0, fmt.Errorf("check current version: %w", err)
	}
	req := protocol.PushRequest{Vault: vault, File: protocol.FilePush{
		Path: name, KeyVersion: keyVersion, Ciphertext: ct, BaseVersion: base}}
	done = onStep(fmt.Sprintf("uploading %s (%.1f KB)", name, float64(len(ct))/1024))
	var resp protocol.PushResponse
	if err := cl.Post("/files/push", req, &resp); err != nil {
		return 0, err
	}
	done(fmt.Sprintf("pushed %s -> version %d", name, resp.Version))
	return resp.Version, nil
}

// ListFiles returns the metadata for every file in a vault.
func ListFiles(vault string) ([]protocol.FileMeta, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var files protocol.FileListResponse
	if err := cl.Get("/files", urlValues("vault", vault), &files); err != nil {
		return nil, err
	}
	return files.Files, nil
}

// PullResult is the outcome of downloading + decrypting a vault file.
type PullResult struct {
	Version int
	OutPath string
}

// Pull downloads and decrypts a file from a vault, writing it to out (or the
// file's own name if out is empty).
func Pull(vault, file, out string, onStep StepFunc) (*PullResult, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	done := onStep("fetching vault key")
	vk, _, err := fetchVaultKey(cl, vault)
	if err != nil {
		return nil, err
	}
	done("got vault key")

	done = onStep("downloading " + file)
	var pr protocol.PullResponse
	if err := cl.Get("/files/pull", urlValues("vault", vault, "path", file), &pr); err != nil {
		return nil, err
	}
	if pr.Deleted {
		return nil, fmt.Errorf("%s is deleted (use checkout --version N to restore)", file)
	}
	done(fmt.Sprintf("downloaded (%.1f KB)", float64(len(pr.Ciphertext))/1024))

	done = onStep("decrypting")
	plain, err := crypto.DecryptBlob(vk, pr.Ciphertext)
	if err != nil {
		return nil, err
	}
	if out == "" {
		out = file
	}
	if err := os.WriteFile(out, plain, 0o600); err != nil {
		return nil, err
	}
	done(fmt.Sprintf("pulled %s (v%d) -> %s", file, pr.Version, out))
	return &PullResult{Version: pr.Version, OutPath: out}, nil
}

// History returns the version history of a vault file.
func History(vault, file string) ([]protocol.HistoryEntry, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var out protocol.HistoryResponse
	if err := cl.Get("/files/history", urlValues("vault", vault, "path", file), &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

// Checkout restores a specific version of a vault file to disk. No step
// callback: the original CLI command has no progress spinner for this action.
func Checkout(vault, file, out string, version int) (*PullResult, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	vk, _, err := fetchVaultKey(cl, vault)
	if err != nil {
		return nil, err
	}
	q := urlValues("vault", vault, "path", file)
	q.Set("version", fmt.Sprintf("%d", version))
	var pr protocol.PullResponse
	if err := cl.Get("/files/pull", q, &pr); err != nil {
		return nil, err
	}
	plain, err := crypto.DecryptBlob(vk, pr.Ciphertext)
	if err != nil {
		return nil, err
	}
	if out == "" {
		out = file
	}
	if err := os.WriteFile(out, plain, 0o600); err != nil {
		return nil, err
	}
	return &PullResult{Version: pr.Version, OutPath: out}, nil
}

// Rm soft-deletes a vault file (history retained). No step callback: the
// original CLI command has no progress spinner for this action.
func Rm(vault, file string) (version int, err error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return 0, err
	}
	vk, keyVersion, err := fetchVaultKey(cl, vault)
	if err != nil {
		return 0, err
	}
	// Soft delete = push an empty, deleted-flagged version. Server marks it.
	base, err := currentVersion(cl, vault, file)
	if err != nil {
		return 0, fmt.Errorf("check current version: %w", err)
	}
	if base == 0 {
		return 0, fmt.Errorf("file %s not found in vault", file)
	}
	ct, err := crypto.EncryptBlob(vk, nil)
	if err != nil {
		return 0, err
	}
	// Reuse push endpoint; deletion flag handled server-side via a dedicated
	// field would be cleaner, but push already versions. We send a tombstone.
	req := protocol.PushRequest{Vault: vault, File: protocol.FilePush{
		Path: file, KeyVersion: keyVersion, Ciphertext: ct, BaseVersion: base, Deleted: true}}
	var resp protocol.PushResponse
	if err := cl.Post("/files/push", req, &resp); err != nil {
		return 0, err
	}
	return resp.Version, nil
}

// Audit returns the audit log entries for a vault.
func Audit(vault string) ([]protocol.AuditEntry, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var out protocol.AuditResponse
	if err := cl.Get("/audit", urlValues("vault", vault), &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}
