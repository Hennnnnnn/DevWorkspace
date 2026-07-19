package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/devsync/devsync/internal/client/config"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get or set client configuration",
	}
	cmd.AddCommand(newConfigSetCmd(), newConfigGetCmd())
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value (e.g. server_url)",
		Long:  "Set a client configuration value.\n\nArguments:\n  <key>    Config key (e.g. server_url)\n  <value>  Config value",
		Args:  expectArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := config.Load()
			if err != nil {
				return err
			}
			key, val := args[0], args[1]
			switch key {
			case "server_url":
				c.ServerURL = val
			default:
				return fmt.Errorf("unknown config key %q", key)
			}
			if err := c.Save(); err != nil {
				return err
			}
			fmt.Printf("set %s = %s\n", key, val)
			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Long:  "Get a client configuration value.\n\nArguments:\n  <key>  Config key (e.g. server_url)",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			c, err := config.Load()
			if err != nil {
				return err
			}
			switch args[0] {
			case "server_url":
				fmt.Println(c.ServerURL)
			default:
				return fmt.Errorf("unknown config key %q", args[0])
			}
			return nil
		},
	}
}
