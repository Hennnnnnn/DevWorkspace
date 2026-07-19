package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/protocol"
)

func newDeviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "device",
		Short: "Manage this account's devices",
	}
	cmd.AddCommand(newDeviceListCmd(), newDeviceRevokeCmd(), newDeviceAddCmd())
	return cmd
}

func newDeviceListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List your devices",
		RunE: func(_ *cobra.Command, _ []string) error {
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			var out protocol.DeviceList
			if err := cl.Get("/devices", nil, &out); err != nil {
				return err
			}
			for _, d := range out.Devices {
				fmt.Printf("%-8s %-10s %-8s %s\n", short(d.ID), d.Name, d.Status, d.Fingerprint)
			}
			return nil
		},
	}
}

func newDeviceRevokeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <device-id>",
		Short: "Revoke one of your devices",
		Long:  "Revoke a device by its ID.\n\nArguments:\n  <device-id>  Device ID (use `devsync device list` to find it)",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			if err := cl.Post("/devices/link", protocol.RevokeRequest{DeviceID: args[0]}, nil); err != nil {
				return err
			}
			fmt.Printf("revoked device %s\n", args[0])
			fmt.Println("REMINDER: rotate vault keys for any vault this device could read (`devsync revoke`).")
			return nil
		},
	}
}

// newDeviceAddCmd explains the second-device linking flow. The new device runs
// `devsync init` + `devsync register`; this command (on the trusted device)
// signs the new device's fingerprint so the server auto-activates it.
func newDeviceAddCmd() *cobra.Command {
	var fingerprint string
	cmd := &cobra.Command{
		Use:   "add --fingerprint <new-device-fp>",
		Short: "Authorize a new device by signing its fingerprint (run on a trusted device)",
		RunE: func(_ *cobra.Command, _ []string) error {
			if fingerprint == "" {
				return fmt.Errorf("--fingerprint of the new device is required")
			}
			// The linking signature is produced during `register` on the new device
			// using data printed here. We print a signature the user copies over.
			return fmt.Errorf("device linking: run `devsync register` on the NEW device; " +
				"it will submit a link signature. Full interactive linking lands with the agent socket. " +
				"For now, an admin can `devsync approve` the new device")
		},
	}
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "new device fingerprint")
	return cmd
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
