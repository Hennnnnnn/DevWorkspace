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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/config"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var dev bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update devsync to the latest release",
		RunE: func(_ *cobra.Command, _ []string) error {
			if dev {
				return updateDev()
			}
			return updateRelease()
		},
	}
	cmd.Flags().BoolVar(&dev, "dev", false, "build from latest commit instead of downloading release")
	return cmd
}

func updateDev() error {
	sp := startSpinner("checking latest commit")
	resp, err := http.Get("https://api.github.com/repos/Hennnnnnn/DevWorkspace/commits/main?per_page=1")
	if err != nil {
		return fmt.Errorf("check commits: %w", err)
	}
	defer resp.Body.Close()
	var commit struct{ Sha string `json:"sha"` }
	if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil {
		return fmt.Errorf("parse commit: %w", err)
	}
	sha := commit.Sha[:7]
	sp.done(fmt.Sprintf("latest commit: %s", sha))

	sp = startSpinner("cloning repo")
	tmp, err := os.MkdirTemp("", "devsync-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := exec.Command("git", "clone", "https://github.com/Hennnnnnn/DevWorkspace.git", tmp, "--quiet").Run(); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	sp.done("cloned")

	sp = startSpinner("building devsync")
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	serverURL := cfg.ServerURL
	if serverURL == "" {
		serverURL = config.DefaultServerURL
	}
	ldflags := fmt.Sprintf("-X github.com/Hennnnnnn/DevWorkspace/internal/client/config.DefaultServerURL=%s -X github.com/Hennnnnnn/DevWorkspace/internal/client/commands.Version=%s", serverURL, sha)
	binName := "devsync"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	outPath := filepath.Join(tmp, binName)
	build := exec.Command("go", "build", "-ldflags", ldflags, "-o", outPath, "./cmd/devsync")
	build.Dir = tmp
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("go build: %s: %w", strings.TrimSpace(string(out)), err)
	}
	sp.done("built")

	sp = startSpinner("installing")
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}

	f, err := os.Open(outPath)
	if err != nil {
		if entries, e := os.ReadDir(tmp); e == nil {
			names := make([]string, 0, len(entries))
			for _, ent := range entries {
				names = append(names, ent.Name())
			}
			return fmt.Errorf("open built binary %s (tmp contents: %s): %w", outPath, strings.Join(names, ", "), err)
		}
		return fmt.Errorf("open built binary %s: %w", outPath, err)
	}
	defer f.Close()
	if err := writeAtomic(exe, f, 0755); err != nil {
		return fmt.Errorf("install: %w", err)
	}
	sp.done("installed")

	fmt.Printf("updated to commit %s\n", sha)
	return nil
}

func updateRelease() error {
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
}

func writeAtomic(dst string, src io.Reader, mode os.FileMode) error {
	old := dst + ".old"
	os.Remove(old)

	if err := os.Rename(dst, old); err == nil {
		// rename succeeded — dst is free, write in place
		tmp := dst + ".tmp"
		f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			os.Rename(old, dst)
			return fmt.Errorf("create temp: %w", err)
		}
		if _, err := io.Copy(f, src); err != nil {
			f.Close()
			os.Remove(tmp)
			os.Rename(old, dst)
			return fmt.Errorf("write temp: %w", err)
		}
		if err := f.Close(); err != nil {
			os.Remove(tmp)
			os.Rename(old, dst)
			return fmt.Errorf("close temp: %w", err)
		}
		if err := os.Rename(tmp, dst); err != nil {
			os.Rename(old, dst)
			return fmt.Errorf("replace: %w", err)
		}
		os.Remove(old)
		return nil
	}

	// ponytail: rename failed (self-update: running exe locked).
	// Fallback: write .new, batch script swaps after process exits.
	newExe := dst + ".new"
	f, err := os.OpenFile(newExe, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("create new: %w", err)
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		os.Remove(newExe)
		return fmt.Errorf("write new: %w", err)
	}
	f.Close()

	bat := filepath.Join(os.TempDir(), "devsync-update.bat")
	script := fmt.Sprintf("@echo off\r\n:loop\r\ntimeout /t 1 /nobreak >nul\r\nmove /y \"%s\" \"%s\" 2>nul\r\nif exist \"%s\" goto loop\r\ndel \"%%~f0\" 2>nul\r\n", newExe, dst, newExe)
	if err := os.WriteFile(bat, []byte(script), 0644); err != nil {
		return fmt.Errorf("write updater script: %w", err)
	}
	cmd := exec.Command("cmd", "/c", bat)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Start()
	return nil
}
