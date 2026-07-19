package commands

import (
	"fmt"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "push <file> --vault <vault>",
		Short: "Encrypt and upload a file to a vault",
		Long:  "Encrypt a local file and upload it to a vault.\n\nArguments:\n  <file>  Path to the local file to push",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, err := actions.Push(args[0], vault, spinnerStep)
			return err
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}

func newPullCmd() *cobra.Command {
	var vault, out string
	cmd := &cobra.Command{
		Use:   "pull [<file>] --vault <vault>",
		Short: "Download and decrypt a file (or list vault files)",
		Long:  "Download and decrypt a file from a vault. Omit <file> to list all files.\n\nArguments:\n  <file>  (Optional) File name in the vault",
		Args:  maxArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				files, err := actions.ListFiles(vault)
				if err != nil {
					return err
				}
				for _, f := range files {
					del := ""
					if f.Deleted {
						del = " (deleted)"
					}
					fmt.Printf("v%-4d %s%s\n", f.LatestVersion, f.Path, del)
				}
				return nil
			}
			_, err := actions.Pull(vault, args[0], out, spinnerStep)
			return err
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	cmd.Flags().StringVar(&out, "out", "", "output path (default: file name)")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}

func newHistoryCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "history <file> --vault <vault>",
		Short: "Show version history of a file",
		Long:  "Show the version history of a file in a vault.\n\nArguments:\n  <file>  File name in the vault",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			entries, err := actions.History(vault, args[0])
			if err != nil {
				return err
			}
			for _, e := range entries {
				del := ""
				if e.Deleted {
					del = " (deleted)"
				}
				fmt.Printf("v%-4d %s  key=v%d  %d bytes%s\n", e.Version, e.CreatedAt, e.KeyVersion, e.SizeBytes, del)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}

func newCheckoutCmd() *cobra.Command {
	var vault, out string
	var version int
	cmd := &cobra.Command{
		Use:   "checkout <file> --version N --vault <vault>",
		Short: "Restore a specific version of a file to disk",
		Long:  "Restore a specific version of a vault file to disk.\n\nArguments:\n  <file>  File name in the vault",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			res, err := actions.Checkout(vault, args[0], out, version)
			if err != nil {
				return err
			}
			fmt.Printf("checked out %s v%d -> %s\n", args[0], res.Version, res.OutPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	cmd.Flags().StringVar(&out, "out", "", "output path")
	cmd.Flags().IntVar(&version, "version", 0, "version to restore")
	_ = cmd.MarkFlagRequired("vault")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newRmCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "rm <file> --vault <vault>",
		Short: "Soft-delete a file (history retained)",
		Long:  "Soft-delete a vault file. History is retained for recovery.\n\nArguments:\n  <file>  File name in the vault to delete",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			version, err := actions.Rm(vault, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("soft-deleted %s (tombstone v%d)\n", args[0], version)
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}

func newAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit <vault>",
		Short: "Show the audit log for a vault",
		Long:  "Show the audit log (all actions) for a vault.\n\nArguments:\n  <vault>  Vault name",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			entries, err := actions.Audit(args[0])
			if err != nil {
				return err
			}
			for _, e := range entries {
				fmt.Printf("%s  %-12s %-20s %s\n", e.CreatedAt, e.Action, e.Username, e.Target)
			}
			return nil
		},
	}
}
