//go:build windows

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func replaceSelf(newExe, dst string) error {
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
