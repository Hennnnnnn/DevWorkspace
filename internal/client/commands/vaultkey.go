package commands

import (
	"fmt"
	"net/url"

	"github.com/devsync/devsync/internal/client/agent"
	"github.com/devsync/devsync/internal/client/api"
	"github.com/devsync/devsync/internal/crypto"
	"github.com/devsync/devsync/internal/protocol"
)

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
