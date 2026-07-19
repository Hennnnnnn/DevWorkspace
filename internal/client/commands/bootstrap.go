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
		Short: "Promote yourself to admin (first-user bootstrap, no admin needed)",
		Long:  "Call this after register. Promotes your pending user to admin.\nOnly works when no admin exists yet on the server.",
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

			fmt.Printf("bootstrapping admin %s on %s ...\n", cfg.Username, cfg.ServerURL)
			if err := api.PostUnsigned(cfg.ServerURL, "/admin/bootstrap", req, nil); err != nil {
				return err
			}
			fmt.Printf("admin bootstrap successful — you are now admin\n")
			return nil
		},
	}
}
