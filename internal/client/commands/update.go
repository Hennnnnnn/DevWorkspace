package commands

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update devsync to the latest release",
		RunE: func(_ *cobra.Command, _ []string) error {
			sp := startSpinner("checking for updates")

			releaseURL := fmt.Sprintf("https://api.github.com/repos/Hennnnnnn/DevWorkspace/releases/latest")
			resp, err := http.Get(releaseURL)
			if err != nil {
				return fmt.Errorf("check release: %w", err)
			}
			defer resp.Body.Close()

			var tag struct{ TagName string `json:"tag_name"` }
			if err := json.NewDecoder(resp.Body).Decode(&tag); err != nil {
				return fmt.Errorf("parse release: %w", err)
			}

			sp.done(fmt.Sprintf("latest: %s", tag.TagName))

			sp = startSpinner("downloading update")
			ext := ".tar.gz"
			if runtime.GOOS == "windows" {
				ext = ".zip"
			}
			downloadURL := fmt.Sprintf("https://github.com/Hennnnnnn/DevWorkspace/releases/download/%s/devsync_%s_%s%s",
				tag.TagName, runtime.GOOS, runtime.GOARCH, ext)

			dlResp, err := http.Get(downloadURL)
			if err != nil {
				return fmt.Errorf("download: %w", err)
			}
			defer dlResp.Body.Close()
			if dlResp.StatusCode != 200 {
				return fmt.Errorf("download failed: %d", dlResp.StatusCode)
			}

			body, err := io.ReadAll(dlResp.Body)
			if err != nil {
				return fmt.Errorf("read download: %w", err)
			}
			sp.done(fmt.Sprintf("downloaded (%.1f MB)", float64(len(body))/1024/1024))

			sp = startSpinner("extracting")
			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("get executable path: %w", err)
			}
			exeDir := filepath.Dir(exe)

			binName := "devsync"
			srvName := "devsync-server"
			if runtime.GOOS == "windows" {
				binName += ".exe"
				srvName += ".exe"
			}

			if runtime.GOOS == "windows" {
				zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
				if err != nil {
					return fmt.Errorf("read zip: %w", err)
				}
				for _, f := range zr.File {
					name := filepath.Base(f.Name)
					if name != binName && name != srvName {
						continue
					}
					rc, err := f.Open()
					if err != nil {
						return fmt.Errorf("open %s: %w", name, err)
					}
					dst := filepath.Join(exeDir, name)
					if err := writeAtomic(dst, rc, 0755); err != nil {
						return fmt.Errorf("write %s: %w", name, err)
					}
					rc.Close()
				}
			} else {
				gr, err := gzip.NewReader(bytes.NewReader(body))
				if err != nil {
					return fmt.Errorf("read gzip: %w", err)
				}
				tr := tar.NewReader(gr)
				for {
					hdr, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						return fmt.Errorf("read tar: %w", err)
					}
					name := filepath.Base(hdr.Name)
					if name != binName && name != srvName {
						continue
					}
					dst := filepath.Join(exeDir, name)
					if err := writeAtomic(dst, tr, 0755); err != nil {
						return fmt.Errorf("write %s: %w", name, err)
					}
				}
				gr.Close()
			}
			sp.done("extracted")

			fmt.Printf("updated to %s in %s\n", tag.TagName, exeDir)
			return nil
		},
	}
}

func writeAtomic(dst string, src io.Reader, mode os.FileMode) error {
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
