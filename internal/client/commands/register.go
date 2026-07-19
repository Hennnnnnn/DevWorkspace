package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/devsync/devsync/internal/client/api"
	"github.com/devsync/devsync/internal/client/config"
	"github.com/devsync/devsync/internal/client/keystore"
	"github.com/devsync/devsync/internal/crypto"
	"github.com/devsync/devsync/internal/protocol"
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
			if deviceName == "" {
				host, _ := os.Hostname()
				deviceName = host
			}
			pass, err := promptPassphrase("Device passphrase: ")
			if err != nil {
				return err
			}
			kp, err := keystore.Unlock(pass)
			if err != nil {
				return err
			}

			req := protocol.RegisterRequest{
				Username:    username,
				DeviceName:  deviceName,
				SignPubKey:  kp.SignPub,
				BoxPubKey:   kp.BoxPub[:],
				Fingerprint: crypto.Fingerprint(kp.SignPub),
			}
			var resp protocol.RegisterResponse
			if err := api.PostUnsigned(cfg.ServerURL, "/register", req, &resp); err != nil {
				return err
			}
			// Persist identity for future signed requests.
			cfg.Username = username
			cfg.DeviceID = resp.DeviceID
			if err := cfg.Save(); err != nil {
				return err
			}
			fmt.Printf("registered as %q — status: %s\n", username, resp.Status)
			if resp.Status == "pending" {
				fmt.Printf("Ask an admin to approve you:\n  devsync approve %s --fingerprint %s\n",
					username, req.Fingerprint)
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			var resp protocol.WhoAmIResponse
			if err := cl.Get("/whoami", nil, &resp); err != nil {
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
