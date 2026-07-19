package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devsync/devsync/internal/client/agent"
	"github.com/devsync/devsync/internal/crypto"
	"github.com/devsync/devsync/internal/protocol"
)

func newCreateVaultCmd() *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "create-vault <name> --team <team>",
		Short: "Create a vault and seal its key to your devices (admin)",
		Long:  "Create a new encrypted vault.\n\nArguments:\n  <name>  Name of the vault to create",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			kp, err := agent.Get()
			if err != nil {
				return err
			}
			// Fetch caller's own devices to seal the new key to each.
			var devs protocol.DeviceList
			if err := cl.Get("/devices", nil, &devs); err != nil {
				return err
			}
			vk, err := crypto.NewVaultKey()
			if err != nil {
				return err
			}
			shares, err := sealToDevices(vk, 1, devs.Devices, kp)
			if err != nil {
				return err
			}
			var v protocol.Vault
			req := protocol.CreateVaultRequest{Team: team, Name: args[0], Shares: shares}
			if err := cl.Post("/admin/create-vault", req, &v); err != nil {
				return err
			}
			fmt.Printf("vault %q created in team %q\n", v.Name, team)
			return nil
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "team name")
	_ = cmd.MarkFlagRequired("team")
	return cmd
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

func newGrantCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "grant <user> --vault <vault>",
		Short: "Grant a user access to a vault (admin, re-seals the key)",
		Long:  "Grant an existing user access to a vault.\n\nArguments:\n  <user>  Username of the grantee",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := authedClient()
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
			devs, err := granteeDevices(cl, args[0])
			if err != nil {
				return err
			}
			shares, err := sealToDevices(vk, keyVersion, devs, kp)
			if err != nil {
				return err
			}
			req := protocol.GrantRequest{Username: args[0], Vault: vault, Shares: shares}
			if err := cl.Post("/admin/grant", req, nil); err != nil {
				return err
			}
			fmt.Printf("granted %s access to vault %q\n", args[0], vault)
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}

func newRevokeCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "revoke <user> --vault <vault>",
		Short: "Revoke a user's vault access and rotate the key (admin)",
		Long:  "Revoke a user's access and rotate the vault key.\n\nArguments:\n  <user>  Username to revoke",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			kp, err := agent.Get()
			if err != nil {
				return err
			}
			// Rotate: new key, re-encrypt all files, re-seal to remaining devices.
			oldKey, _, err := fetchVaultKey(cl, vault)
			if err != nil {
				return err
			}
			newKey, err := crypto.NewVaultKey()
			if err != nil {
				return err
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
				return err
			}
			shares, err := sealToDevices(newKey, newVer, mine.Devices, kp)
			if err != nil {
				return err
			}

			// Re-encrypt every file under the new key.
			var files protocol.FileListResponse
			if err := cl.Get("/files", urlValues("vault", vault), &files); err != nil {
				return err
			}
			var reFiles []protocol.FilePush
			for _, f := range files.Files {
				if f.Deleted {
					continue
				}
				var pr protocol.PullResponse
				if err := cl.Get("/files/pull", urlValues("vault", vault, "path", f.Path), &pr); err != nil {
					return err
				}
				plain, err := crypto.DecryptBlob(oldKey, pr.Ciphertext)
				if err != nil {
					return fmt.Errorf("decrypt %s: %w", f.Path, err)
				}
				ct, err := crypto.EncryptBlob(newKey, plain)
				if err != nil {
					return err
				}
				reFiles = append(reFiles, protocol.FilePush{
					Path: f.Path, KeyVersion: newVer, Ciphertext: ct, BaseVersion: f.LatestVersion})
			}

			req := protocol.RevokeRequest{
				Username: args[0], Vault: vault,
				NewKeyVersion: newVer, Shares: shares, Files: reFiles}
			if err := cl.Post("/admin/revoke", req, nil); err != nil {
				return err
			}
			fmt.Printf("revoked %s from %q, rotated to key v%d (%d files re-encrypted)\n",
				args[0], vault, newVer, len(reFiles))
			fmt.Println("REMINDER: rotate the underlying secrets too — run `devsync audit " + vault + "` to see which files this user could read.")
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}
