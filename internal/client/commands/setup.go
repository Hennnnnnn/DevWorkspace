package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/agent"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/api"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/keystore"
)

func newSetupCmd() *cobra.Command {
	var username, team, vault, file string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive guide: init → register → push your first secret",
		Long: `Walks you through the entire initial setup step by step.

Steps:
  1. Generate a device keypair (init)
  2. Register with the server (register)
  3. Become admin (bootstrap-admin)
  4. Create a team (create-team)
  5. Create a vault (create-vault)
  6. Push a secret (push)

Flags let you skip prompts if you already know the values.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			if cfg == nil {
				cfg = &config.Config{}
			}

			// --- step 1: init ---
			if !keystore.Exists() {
				fmt.Println("\n── Step 1: Generate a device keypair ──")
				fmt.Println("This creates your device identity. You'll need a passphrase.")
				if err := newInitCmd().RunE(cmd, args); err != nil {
					return fmt.Errorf("init: %w", err)
				}
			} else {
				fmt.Println("✓ Device key already exists (devsync init done)")
			}

			// --- step 2: register ---
			if cfg.Username == "" {
				if username == "" {
					username, _ = promptLine("Username: ")
				}
				fmt.Println("\n── Step 2: Register with the server ──")
				regCmd := newRegisterCmd()
				regCmd.Flags().Set("username", username)
				if err := regCmd.RunE(cmd, []string{}); err != nil {
					return fmt.Errorf("register: %w", err)
				}
				cfg.Username = username
			} else {
				fmt.Printf("✓ Registered as %s\n", cfg.Username)
			}

			// --- step 3: bootstrap-admin ---
			isAdmin := false
			var aErr error
			if cfg.Username != "" {
				var cl *api.Client
				cl, _, aErr = actions.AuthedClient()
				if aErr == nil {
					var who struct {
						IsAdmin bool `json:"is_admin"`
					}
					cl.Get("/whoami", nil, &who)
					isAdmin = who.IsAdmin
				}
			}
		if !isAdmin {
			fmt.Println("\n── Step 3: Become admin ──")
			fmt.Println("Attempting first-user bootstrap (skipped if admin already exists)...")
			if err := newBootstrapAdminCmd().RunE(cmd, args); err != nil {
				fmt.Printf("(Bootstrap skipped — admin may already exist: %v)\n", err)
			}
		} else {
				fmt.Println("✓ Already admin")
			}

			// --- step 4: unlock ---
			_, err := agent.Get()
			if err != nil {
				fmt.Println("\n── Step 4: Unlock your device key ──")
				if err := newUnlockCmd().RunE(cmd, args); err != nil {
					return fmt.Errorf("unlock: %w", err)
				}
			} else {
				fmt.Println("✓ Device key unlocked")
			}

			// --- step 5: create-team ---
			if team == "" {
				team, _ = promptLine("Team name (e.g. eng): ")
			}
			fmt.Println("\n── Step 5: Create a team ──")
			if err := newCreateTeamCmd().RunE(cmd, []string{team}); err != nil {
				return fmt.Errorf("create-team: %w", err)
			}

			// --- step 6: create-vault ---
			if vault == "" {
				vault, _ = promptLine("Vault name (e.g. secrets): ")
			}
			fmt.Println("\n── Step 6: Create a vault ──")
			vaultCmd := newCreateVaultCmd()
			vaultCmd.Flags().Set("team", team)
			if err := vaultCmd.RunE(cmd, []string{vault}); err != nil {
				return fmt.Errorf("create-vault: %w", err)
			}

			// --- step 7: push ---
			if file == "" {
				file, _ = promptLine("File to push (e.g. .env): ")
			}
			fmt.Println("\n── Step 7: Push your first secret ──")
			pushCmd := newPushCmd()
			pushCmd.Flags().Set("vault", vault)
			if err := pushCmd.RunE(cmd, []string{file}); err != nil {
				return fmt.Errorf("push: %w", err)
			}

			fmt.Println("\n✅ Setup complete!")
			fmt.Println("Next: share with your team:")
			fmt.Println("  devsync invite <user> --team", team)
			fmt.Println("  devsync grant <user> --vault", vault)
			fmt.Println("\nOther commands:")
			fmt.Println("  devsync pull .env --vault", vault)
			fmt.Println("  devsync history .env --vault", vault)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "username (skip prompt)")
	cmd.Flags().StringVar(&team, "team", "", "team name (skip prompt)")
	cmd.Flags().StringVar(&vault, "vault", "", "vault name (skip prompt)")
	cmd.Flags().StringVar(&file, "file", "", "file to push (skip prompt)")
	return cmd
}
