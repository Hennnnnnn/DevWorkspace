package actions

import (
	"fmt"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/agent"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// ListVaults returns the vaults the caller has access to.
func ListVaults() ([]protocol.Vault, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	var out protocol.VaultList
	if err := cl.Get("/vaults", nil, &out); err != nil {
		return nil, err
	}
	return out.Vaults, nil
}

// CreateVault creates a vault and seals its key to the caller's own devices (admin).
func CreateVault(name, team string) (*protocol.Vault, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	kp, err := agent.Get()
	if err != nil {
		return nil, err
	}
	// Fetch caller's own devices to seal the new key to each.
	var devs protocol.DeviceList
	if err := cl.Get("/devices", nil, &devs); err != nil {
		return nil, err
	}
	vk, err := crypto.NewVaultKey()
	if err != nil {
		return nil, err
	}
	shares, err := sealToDevices(vk, 1, devs.Devices, kp)
	if err != nil {
		return nil, err
	}
	var v protocol.Vault
	req := protocol.CreateVaultRequest{Team: team, Name: name, Shares: shares}
	if err := cl.Post("/teams/vaults/create", req, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// Grant gives a user access to a vault, re-sealing the key to their devices (team admin).
func Grant(user, vault, team string) error {
	cl, _, err := AuthedClient()
	if err != nil {
		return err
	}
	kp, err := agent.Get()
	if err != nil {
		return err
	}
	// Admin must hold the vault key to re-seal it.
	vk, keyVersion, err := fetchVaultKey(cl, vault)
	if err != nil {
		return err
	}
	// Find grantee's active devices. Reuse members-by-team? Simpler: the
	// server exposes device box keys via /members; grantee must be an
	// active user with active devices. We look them up via /members needs
	// team; instead we use a dedicated approach: ask for the user's devices
	// through members of any shared team is complex — use fingerprints from
	// `members`. Here we require the grantee already approved (active).
	devs, err := granteeDevices(cl, user)
	if err != nil {
		return err
	}
	shares, err := sealToDevices(vk, keyVersion, devs, kp)
	if err != nil {
		return err
	}
	req := protocol.GrantRequest{Username: user, Vault: vault, Team: team, Shares: shares}
	return cl.Post("/teams/vaults/grant", req, nil)
}

// RevokeResult is the outcome of rotating a vault's key after revoking a user.
type RevokeResult struct {
	NewKeyVersion    int
	FilesReEncrypted int
}

// Revoke revokes a user's vault access and rotates the vault key (team admin).
func Revoke(user, vault, team string) (*RevokeResult, error) {
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	kp, err := agent.Get()
	if err != nil {
		return nil, err
	}
	// Rotate: new key, re-encrypt all files, re-seal to remaining devices.
	oldKey, _, err := fetchVaultKey(cl, vault)
	if err != nil {
		return nil, err
	}
	newKey, err := crypto.NewVaultKey()
	if err != nil {
		return nil, err
	}
	// New key version = old highest + 1.
	_, oldVer, _ := fetchVaultKey(cl, vault)
	newVer := oldVer + 1

	// Remaining devices = current caller's devices + other grant-holders.
	// Simplification: re-seal to caller's own devices; other members
	// re-share via `grant` again. ponytail: full survivor set requires a
	// server endpoint listing all granted devices. Upgrade: add
	// GET /vaults/devices?vault= to enumerate survivor device box keys.
	var mine protocol.DeviceList
	if err := cl.Get("/devices", nil, &mine); err != nil {
		return nil, err
	}
	shares, err := sealToDevices(newKey, newVer, mine.Devices, kp)
	if err != nil {
		return nil, err
	}

	// Re-encrypt every file under the new key.
	var files protocol.FileListResponse
	if err := cl.Get("/files", urlValues("vault", vault), &files); err != nil {
		return nil, err
	}
	var reFiles []protocol.FilePush
	for _, f := range files.Files {
		if f.Deleted {
			continue
		}
		var pr protocol.PullResponse
		if err := cl.Get("/files/pull", urlValues("vault", vault, "path", f.Path), &pr); err != nil {
			return nil, err
		}
		plain, err := crypto.DecryptBlob(oldKey, pr.Ciphertext)
		if err != nil {
			return nil, fmt.Errorf("decrypt %s: %w", f.Path, err)
		}
		ct, err := crypto.EncryptBlob(newKey, plain)
		if err != nil {
			return nil, err
		}
		reFiles = append(reFiles, protocol.FilePush{
			Path: f.Path, KeyVersion: newVer, Ciphertext: ct, BaseVersion: f.LatestVersion})
	}

	req := protocol.RevokeRequest{
		Username: user, Vault: vault, Team: team,
		NewKeyVersion: newVer, Shares: shares, Files: reFiles}
	if err := cl.Post("/teams/vaults/revoke", req, nil); err != nil {
		return nil, err
	}
	return &RevokeResult{NewKeyVersion: newVer, FilesReEncrypted: len(reFiles)}, nil
}
