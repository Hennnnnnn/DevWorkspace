package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/devsync/devsync/internal/client/api"
	"github.com/devsync/devsync/internal/crypto"
	"github.com/devsync/devsync/internal/protocol"
	"github.com/spf13/cobra"
)

const maxPlaintext = 1 << 20 // 1 MB

func newPushCmd() *cobra.Command {
	var vault string
	cmd := &cobra.Command{
		Use:   "push <file> --vault <vault>",
		Short: "Encrypt and upload a file to a vault",
		Long:  "Encrypt a local file and upload it to a vault.\n\nArguments:\n  <file>  Path to the local file to push",
		Args:  expectArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			plain, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			if len(plain) > maxPlaintext {
				return fmt.Errorf("file exceeds 1MB limit (%d bytes)", len(plain))
			}
			vk, keyVersion, err := fetchVaultKey(cl, vault)
			if err != nil {
				return err
			}
			ct, err := crypto.EncryptBlob(vk, plain)
			if err != nil {
				return err
			}
			name := filepath.Base(args[0])

			// Determine base version (0 if new) for optimistic lock.
			base := currentVersion(cl, vault, name)
			req := protocol.PushRequest{Vault: vault, File: protocol.FilePush{
				Path: name, KeyVersion: keyVersion, Ciphertext: ct, BaseVersion: base}}
			var resp protocol.PushResponse
			if err := cl.Post("/files/push", req, &resp); err != nil {
				return err
			}
			fmt.Printf("pushed %s -> version %d\n", name, resp.Version)
			return nil
		},
	}
	cmd.Flags().StringVar(&vault, "vault", "", "vault name")
	_ = cmd.MarkFlagRequired("vault")
	return cmd
}

// currentVersion returns the latest version of a file, or 0 if absent.
func currentVersion(cl *api.Client, vault, path string) int {
	var files protocol.FileListResponse
	if err := cl.Get("/files", urlValues("vault", vault), &files); err != nil {
		return 0
	}
	for _, f := range files.Files {
		if f.Path == path {
			return f.LatestVersion
		}
	}
	return 0
}

func newPullCmd() *cobra.Command {
	var vault, out string
	cmd := &cobra.Command{
		Use:   "pull [<file>] --vault <vault>",
		Short: "Download and decrypt a file (or list vault files)",
		Long:  "Download and decrypt a file from a vault. Omit <file> to list all files.\n\nArguments:\n  <file>  (Optional) File name in the vault",
		Args:  maxArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				var files protocol.FileListResponse
				if err := cl.Get("/files", urlValues("vault", vault), &files); err != nil {
					return err
				}
				for _, f := range files.Files {
					del := ""
					if f.Deleted {
						del = " (deleted)"
					}
					fmt.Printf("v%-4d %s%s\n", f.LatestVersion, f.Path, del)
				}
				return nil
			}
			vk, _, err := fetchVaultKey(cl, vault)
			if err != nil {
				return err
			}
			var pr protocol.PullResponse
			if err := cl.Get("/files/pull", urlValues("vault", vault, "path", args[0]), &pr); err != nil {
				return err
			}
			if pr.Deleted {
				return fmt.Errorf("%s is deleted (use checkout --version N to restore)", args[0])
			}
			plain, err := crypto.DecryptBlob(vk, pr.Ciphertext)
			if err != nil {
				return err
			}
			if out == "" {
				out = args[0]
			}
			if err := os.WriteFile(out, plain, 0o600); err != nil {
				return err
			}
			fmt.Printf("pulled %s (v%d) -> %s\n", args[0], pr.Version, out)
			return nil
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			var out protocol.HistoryResponse
			if err := cl.Get("/files/history", urlValues("vault", vault, "path", args[0]), &out); err != nil {
				return err
			}
			for _, e := range out.Entries {
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			vk, _, err := fetchVaultKey(cl, vault)
			if err != nil {
				return err
			}
			q := urlValues("vault", vault, "path", args[0])
			q.Set("version", fmt.Sprintf("%d", version))
			var pr protocol.PullResponse
			if err := cl.Get("/files/pull", q, &pr); err != nil {
				return err
			}
			plain, err := crypto.DecryptBlob(vk, pr.Ciphertext)
			if err != nil {
				return err
			}
			if out == "" {
				out = args[0]
			}
			if err := os.WriteFile(out, plain, 0o600); err != nil {
				return err
			}
			fmt.Printf("checked out %s v%d -> %s\n", args[0], pr.Version, out)
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			vk, keyVersion, err := fetchVaultKey(cl, vault)
			if err != nil {
				return err
			}
			// Soft delete = push an empty, deleted-flagged version. Server marks it.
			base := currentVersion(cl, vault, args[0])
			if base == 0 {
				return fmt.Errorf("file %s not found in vault", args[0])
			}
			ct, err := crypto.EncryptBlob(vk, nil)
			if err != nil {
				return err
			}
			// Reuse push endpoint; deletion flag handled server-side via a dedicated
			// field would be cleaner, but push already versions. We send a tombstone.
			req := protocol.PushRequest{Vault: vault, File: protocol.FilePush{
				Path: args[0], KeyVersion: keyVersion, Ciphertext: ct, BaseVersion: base, Deleted: true}}
			var resp protocol.PushResponse
			if err := cl.Post("/files/push", req, &resp); err != nil {
				return err
			}
			fmt.Printf("soft-deleted %s (tombstone v%d)\n", args[0], resp.Version)
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
			cl, _, err := authedClient()
			if err != nil {
				return err
			}
			var out protocol.AuditResponse
			if err := cl.Get("/audit", urlValues("vault", args[0]), &out); err != nil {
				return err
			}
			for _, e := range out.Entries {
				fmt.Printf("%s  %-12s %-20s %s\n", e.CreatedAt, e.Action, e.Username, e.Target)
			}
			return nil
		},
	}
}
