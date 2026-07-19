package actions

import (
	"fmt"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/agent"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/api"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

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

// sealSharesFor seals a vault key to devices, stamping the vault ID.
func sealSharesFor(vk crypto.VaultKey, keyVersion int, vaultID string, devices []protocol.Device, kp *crypto.KeyPair) ([]protocol.VaultKeyShare, error) {
	shares, err := sealToDevices(vk, keyVersion, devices, kp)
	if err != nil {
		return nil, err
	}
	for i := range shares {
		shares[i].VaultID = vaultID
	}
	return shares, nil
}

// ApproveResult reports what happened after approving a pending user.
type ApproveResult struct {
	// ShareNote is set when device enumeration failed after a successful
	// approval — approval still succeeded, sharing is best-effort.
	ShareNote string
	// VaultsShared is how many vault keys were re-shared to the user's devices.
	VaultsShared int
}

// Approve approves a pending user after their fingerprint has been verified
// out-of-band, then re-shares every vault key the admin holds to the user's
// devices (best-effort).
func Approve(user, fingerprint string) (*ApproveResult, error) {
	if fingerprint == "" {
		return nil, fmt.Errorf("fingerprint is required; confirm it out-of-band with the user")
	}
	cl, _, err := AuthedClient()
	if err != nil {
		return nil, err
	}
	kp, err := agent.Get()
	if err != nil {
		return nil, err
	}

	// After activation the user's device can receive sealed vault keys.
	// Seal every vault the admin holds to the new device. We approve first
	// (activates the device so it appears in /users/devices), then re-share.
	req := protocol.ApproveRequest{Username: user, Fingerprint: fingerprint}
	if err := cl.Post("/admin/approve", req, nil); err != nil {
		return nil, err
	}

	// Re-share: for each vault the admin can open, seal to the new devices.
	devs, err := granteeDevices(cl, user)
	if err != nil {
		// Approval succeeded; sharing is best-effort with guidance.
		return &ApproveResult{ShareNote: fmt.Sprintf("could not enumerate %s's devices for key sharing: %v", user, err)}, nil
	}
	var vaults protocol.VaultList
	if err := cl.Get("/vaults", nil, &vaults); err != nil {
		return nil, err
	}
	shared := 0
	for _, v := range vaults.Vaults {
		vk, keyVersion, err := fetchVaultKey(cl, v.Name)
		if err != nil {
			continue // admin doesn't hold this one
		}
		shares, err := sealSharesFor(vk, keyVersion, v.ID, devs, kp)
		if err != nil || len(shares) == 0 {
			continue
		}
		gr := protocol.GrantRequest{Username: user, Vault: v.Name, Shares: shares}
		if err := cl.Post("/admin/grant", gr, nil); err == nil {
			shared++
		}
	}
	return &ApproveResult{VaultsShared: shared}, nil
}
