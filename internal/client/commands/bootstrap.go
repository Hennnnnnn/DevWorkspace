package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/api"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
)

func newBootstrapAdminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bootstrap-admin",
		Short: "Activate your account (first-user bootstrap, no admin needed)",
		Long:  "Call this after register. Activates your pending user.\nOnly works when no active users exist yet on the server.",
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Username == "" || cfg.ServerURL == "" {
				return fmt.Errorf("register first — no username or server_url in config")
			}

			pass, err := promptPassphrase("Device passphrase: ")
			if err != nil {
				return err
			}
			kp, err := keystore.Unlock(pass)
			if err != nil {
				return err
			}

			fp := crypto.Fingerprint(kp.SignPub)
			req := map[string]string{
				"username":    cfg.Username,
				"fingerprint": fp,
			}

			fmt.Printf("activating %s on %s ...\n", cfg.Username, cfg.ServerURL)
			if err := api.PostUnsigned(cfg.ServerURL, "/bootstrap", req, nil); err != nil {
				return err
			}
			fmt.Printf("account activated — you can now create a team\n")
			return nil
		},
	}
}
