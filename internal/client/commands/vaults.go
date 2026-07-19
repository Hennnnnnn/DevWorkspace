package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

func newCreateVaultCmd() *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "create-vault <name> --team <team>",
		Short: "Create a vault and seal its key to your devices (admin)",
		Long:  "Create a new encrypted vault.\n\nArguments:\n  <name>  Name of the vault to create",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			v, err := actions.CreateVault(args[0], team)
			if err != nil {
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

func newGrantCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "grant <user> --vault <vault>",
		Short: "Grant a user access to a vault (admin, re-seals the key)",
		Long:  "Grant an existing user access to a vault.\n\nArguments:\n  <user>  Username of the grantee",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := actions.Grant(args[0], vault); err != nil {
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
			res, err := actions.Revoke(args[0], vault)
			if err != nil {
				return err
			}
			fmt.Printf("revoked %s from %q, rotated to key v%d (%d files re-encrypted)\n",
				args[0], vault, res.NewKeyVersion, res.FilesReEncrypted)
			fmt.Println("REMINDER: rotate the underlying secrets too — run `devsync audit " + vault + "` to see which files this user could read.")
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}
