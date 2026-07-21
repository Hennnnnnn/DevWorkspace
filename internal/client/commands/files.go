package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "push <file> [--vault <vault>]",
		Short: "Encrypt and upload a file to a vault (push = encrypt & upload)",
		Long:  "Encrypt a local file and upload it to a vault.\n\nArguments:\n  <file>  Path to the local file to push",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			v, err := resolveVault(vault)
			if err != nil {
				return err
			}
			_, err = actions.Push(args[0], v, spinnerStep)
			return err
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name (auto-detected if you have one vault)")
	return cmd
}

func newPullCmd() *cobra.Command {
	var vault, out string
	var list bool
	cmd := &cobra.Command{
		Use:   "pull [<file>] [--vault <vault>]",
		Short: "Download and decrypt a file (pull = download & decrypt)",
		Long:  "Download and decrypt a file from a vault. Omit <file> to pick from the vault's files (--list to just list them).\n\nArguments:\n  <file>  (Optional) File name in the vault",
		Args:  maxArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			v, err := resolveVault(vault)
			if err != nil {
				return err
			}
			file := ""
			if len(args) > 0 {
				file = args[0]
			}
			if list { // old "pull with no args lists files" behavior
				return listVaultFiles(v)
			}
			if file == "" {
				file, err = resolveFile(v)
				if err != nil {
					return err
				}
				// Chosen implicitly — never overwrite a local file silently.
				dst := out
				if dst == "" {
					dst = file
				}
				if _, statErr := os.Stat(dst); statErr == nil {
					if !isTTY() {
						return fmt.Errorf("%s exists locally, refusing to overwrite (pass <file> and --out explicitly)", dst)
					}
					ans, err := promptLine(fmt.Sprintf("%s exists locally, overwrite? [y/N]: ", dst))
					if err != nil {
						return err
					}
					if !strings.EqualFold(ans, "y") && !strings.EqualFold(ans, "yes") {
						return fmt.Errorf("aborted")
					}
				}
			}
			_, err = actions.Pull(v, file, out, spinnerStep)
			return err
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name (auto-detected if you have one vault)")
	cmd.Flags().StringVar(&out, "out", "", "output path (default: file name)")
	cmd.Flags().BoolVar(&list, "list", false, "list vault files instead of pulling")
	return cmd
}

func listVaultFiles(vault string) error {
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

// resolveFile picks the file to pull when none was named: the only live file
// if there is exactly one, otherwise a picker (TTY) or an error listing the
// files (non-TTY).
func resolveFile(vault string) (string, error) {
	files, err := actions.ListFiles(vault)
	if err != nil {
		return "", err
	}
	var names []string
	for _, f := range files {
		if !f.Deleted {
			names = append(names, f.Path)
		}
	}
	switch len(names) {
	case 0:
		return "", fmt.Errorf("vault %s has no files", vault)
	case 1:
		return names[0], nil
	}
	if !isTTY() {
		return "", fmt.Errorf("multiple files, pass a file name: %s", strings.Join(names, ", "))
	}
	return pickFromList("file", names)
}

func newHistoryCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "history <file> [--vault <vault>]",
		Short: "Show version history of a file",
		Long:  "Show the version history of a file in a vault.\n\nArguments:\n  <file>  File name in the vault",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			v, err := resolveVault(vault)
			if err != nil {
				return err
			}
			entries, err := actions.History(v, args[0])
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
	cmd.Flags().StringVar(&vault, "vault", "", "vault name (auto-detected if you have one vault)")
	return cmd
}

func newCheckoutCmd() *cobra.Command {
	var vault, out string
	var version int
	cmd := &cobra.Command{
		Use:   "checkout <file> --version N [--vault <vault>]",
		Short: "Restore a specific version of a file to disk",
		Long:  "Restore a specific version of a vault file to disk.\n\nArguments:\n  <file>  File name in the vault",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			v, err := resolveVault(vault)
			if err != nil {
				return err
			}
			res, err := actions.Checkout(v, args[0], out, version)
			if err != nil {
				return err
			}
			fmt.Printf("checked out %s v%d -> %s\n", args[0], res.Version, res.OutPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name (auto-detected if you have one vault)")
	cmd.Flags().StringVar(&out, "out", "", "output path")
	cmd.Flags().IntVar(&version, "version", 0, "version to restore")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newRmCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "rm <file> [--vault <vault>]",
		Short: "Soft-delete a file (history retained)",
		Long:  "Soft-delete a vault file. History is retained for recovery.\n\nArguments:\n  <file>  File name in the vault to delete",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			v, err := resolveVault(vault)
			if err != nil {
				return err
			}
			version, err := actions.Rm(v, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("soft-deleted %s (tombstone v%d)\n", args[0], version)
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name (auto-detected if you have one vault)")
	return cmd
}

func newAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit [<vault>]",
		Short: "Show the audit log for a vault",
		Long:  "Show the audit log (all actions) for a vault.\n\nArguments:\n  <vault>  (Optional) Vault name, auto-detected if you have one vault",
		Args:  maxArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			vault := ""
			if len(args) > 0 {
				vault = args[0]
			}
			v, err := resolveVault(vault)
			if err != nil {
				return err
			}
			entries, err := actions.Audit(v)
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
