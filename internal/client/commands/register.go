package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
)

func newRegisterCmd() *cobra.Command {
	var username, deviceName string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register this device's public key with the server",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.ServerURL == "" {
				return fmt.Errorf("server_url not set — run `devsync config set server_url <url>`")
			}
			if username == "" {
				username, err = promptLine("Username: ")
				if err != nil {
					return err
				}
			}
			pass, err := promptPassphrase("Device passphrase: ")
			if err != nil {
				return err
			}
			res, err := actions.Register(username, deviceName, pass)
			if err != nil {
				return err
			}
			fmt.Printf("registered as %q — status: %s\n", res.Username, res.Status)
			if res.Status == "pending" {
				fmt.Printf("Ask an admin to approve you:\n  devsync approve %s --fingerprint %s\n",
					res.Username, res.Fingerprint)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "username to register")
	cmd.Flags().StringVar(&deviceName, "device", "", "device name (default: hostname)")
	return cmd
}

func newWhoAmICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the current identity and device status",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := actions.WhoAmI()
			if err != nil {
				return err
			}
			admin := ""
			if resp.IsAdmin {
				admin = " (admin)"
			}
			fmt.Printf("%s%s — status: %s\n", resp.Username, admin, resp.Status)
			fmt.Printf("device: %s [%s] %s\n", resp.Device.Name, resp.Device.Status, resp.Device.Fingerprint)
			return nil
		},
	}
}
