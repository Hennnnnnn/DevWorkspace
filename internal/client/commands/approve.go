package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devsync/devsync/internal/client/agent"
	"github.com/devsync/devsync/internal/client/api"
	"github.com/devsync/devsync/internal/crypto"
	"github.com/devsync/devsync/internal/protocol"
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

func newApproveCmd() *cobra.Command {
	var fingerprint string
	cmd := &cobra.Command{
		Use:   "approve <user> --fingerprint <fp>",
		Short: "Approve a pending user after verifying their fingerprint out-of-band (admin)",
		Long:  "Approve a pending user and re-share vault keys to their devices.\n\nArguments:\n  <user>  Username to approve",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if fingerprint == "" {
				return fmt.Errorf("--fingerprint is required; confirm it out-of-band with the user")
			}
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			kp, err := agent.Get()
			if err != nil {
				return err
			}

			// After activation the user's device can receive sealed vault keys.
			// Seal every vault the admin holds to the new device. We approve first
			// (activates the device so it appears in /users/devices), then re-share.
			req := protocol.ApproveRequest{Username: args[0], Fingerprint: fingerprint}
			if err := cl.Post("/admin/approve", req, nil); err != nil {
				return err
			}
			fmt.Printf("approved %s\n", args[0])

			// Re-share: for each vault the admin can open, seal to the new devices.
			devs, err := granteeDevices(cl, args[0])
			if err != nil {
				// Approval succeeded; sharing is best-effort with guidance.
				fmt.Printf("note: could not enumerate %s's devices for key sharing: %v\n", args[0], err)
				return nil
			}
			var vaults protocol.VaultList
			if err := cl.Get("/vaults", nil, &vaults); err != nil {
				return err
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
				gr := protocol.GrantRequest{Username: args[0], Vault: v.Name, Shares: shares}
				if err := cl.Post("/admin/grant", gr, nil); err == nil {
					shared++
				}
			}
			if shared > 0 {
				fmt.Printf("re-shared %d vault key(s) to %s\n", shared, args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "device fingerprint (verify out-of-band)")
	return cmd
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
