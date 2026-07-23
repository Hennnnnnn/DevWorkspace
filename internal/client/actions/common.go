// Package actions holds the business logic behind every devsync CLI command,
// as plain functions with no cobra dependency, so both the CLI and the TUI
// can call the same code.
package actions

import (
	"fmt"
	"net/url"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/agent"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/api"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

// AuthedClient loads config + the unlocked keypair from the agent and returns
// a signed API client.
func AuthedClient() (*api.Client, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	kp, err := agent.Get()
	if err != nil {
		return nil, nil, err
	}
	cl, err := api.New(cfg, kp)
	return cl, cfg, err
}

func urlValues(kv ...string) url.Values {
	v := url.Values{}
	for i := 0; i+1 < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

// fetchVaultKey retrieves and unseals the caller's vault key (highest version).
func fetchVaultKey(cl *api.Client, vault string) (crypto.VaultKey, int, error) {
	var zero crypto.VaultKey
	kp, err := agent.Get()
	if err != nil {
		return zero, 0, err
	}
	var resp protocol.KeySharesResponse
	if err := cl.Get("/vaults/keyshares", urlValues("vault", vault), &resp); err != nil {
		return zero, 0, err
	}
	if len(resp.Shares) == 0 {
		return zero, 0, fmt.Errorf("no vault key available for this device — ask an admin to grant + re-share")
	}
	// Pick the highest key_version this device can open.
	best := -1
	var bestKey crypto.VaultKey
	for _, sh := range resp.Shares {
		vk, err := crypto.OpenVaultKey(sh.EncryptedKey, kp.BoxPub, kp.BoxPriv)
		if err != nil {
			continue
		}
		if sh.KeyVersion > best {
			best = sh.KeyVersion
			bestKey = vk
		}
	}
	if best < 0 {
		return zero, 0, fmt.Errorf("could not decrypt any vault key share (wrong device?)")
	}
	return bestKey, best, nil
}

// currentVersion returns the latest version of a file, or 0 if absent.
func currentVersion(cl *api.Client, vault, path string) (int, error) {
	var files protocol.FileListResponse
	if err := cl.Get("/files", urlValues("vault", vault), &files); err != nil {
		return 0, err
	}
	for _, f := range files.Files {
		if f.Path == path {
			return f.LatestVersion, nil
		}
	}
	return 0, nil
}

// sealToDevices seals a vault key to each active device's box public key.
func sealToDevices(vk crypto.VaultKey, keyVersion int, devices []protocol.Device, _ *crypto.KeyPair) ([]protocol.VaultKeyShare, error) {
	var shares []protocol.VaultKeyShare
	for _, d := range devices {
		if d.Status != "active" || len(d.BoxPubKey) != 32 {
			continue
		}
		var box [32]byte
		copy(box[:], d.BoxPubKey)
		sealed, err := crypto.SealVaultKey(vk, box)
		if err != nil {
			return nil, err
		}
		shares = append(shares, protocol.VaultKeyShare{
			DeviceID: d.ID, KeyVersion: keyVersion, EncryptedKey: sealed})
	}
	if len(shares) == 0 {
		return nil, fmt.Errorf("no active device to seal the vault key to")
	}
	return shares, nil
}

// granteeDevices fetches a user's active devices (box keys) for sealing.
func granteeDevices(cl *api.Client, username string) ([]protocol.Device, error) {
	var out protocol.DeviceList
	if err := cl.Get("/users/devices", urlValues("username", username), &out); err != nil {
		return nil, err
	}
	if len(out.Devices) == 0 {
		return nil, fmt.Errorf("%s has no active devices", username)
	}
	return out.Devices, nil
}
