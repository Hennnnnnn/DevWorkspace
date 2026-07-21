//go:build !windows

package commands

import (
	"fmt"
	"os"
)

func replaceSelf(newExe, _ string) error {
	os.Remove(newExe)
	return fmt.Errorf("self-update: cannot replace running binary; restart after manual swap with %s", newExe)
}
