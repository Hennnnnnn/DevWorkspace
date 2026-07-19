package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
// Example: go build -ldflags "-X github.com/Hennnnnnn/DevWorkspace/internal/client/commands.Version=v0.1.0"
var Version = "dev"

var updateCheckFile string

func init() {
	home, _ := os.UserHomeDir()
	updateCheckFile = filepath.Join(home, ".devsync", ".update-check.json")
}

type updateCache struct {
	Latest string    `json:"latest"`
	Checked time.Time `json:"checked"`
}

func checkUpdate() {
	info, err := os.Stat(updateCheckFile)
	if err == nil && time.Since(info.ModTime()) < 24*time.Hour {
		return
	}

	checkDev := strings.Count(Version, ".") < 2
	var latest string

	if checkDev {
		resp, err := http.Get("https://api.github.com/repos/Hennnnnnn/DevWorkspace/commits/main?per_page=1")
		if err != nil {
			return
		}
		defer resp.Body.Close()
		var commit struct{ Sha string `json:"sha"` }
		if err := json.NewDecoder(resp.Body).Decode(&commit); err != nil || commit.Sha == "" {
			return
		}
		latest = commit.Sha[:7]

		if !strings.HasPrefix(Version, latest) {
			fmt.Fprintf(os.Stderr, "\n⚠ New commits on main: %s\n", Version)
			fmt.Fprintf(os.Stderr, "  Run 'devsync update' (or reinstall) to get the latest.\n\n")
		}
	} else {
		resp, err := http.Get("https://api.github.com/repos/Hennnnnnn/DevWorkspace/releases/latest")
		if err != nil {
			return
		}
		defer resp.Body.Close()
		var rel struct{ TagName string `json:"tag_name"` }
		if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil || rel.TagName == "" {
			return
		}
		latest = rel.TagName

		if Version != "dev" && rel.TagName != Version {
			fmt.Fprintf(os.Stderr, "\n⚠ A new release is available: %s → %s\n", Version, rel.TagName)
			fmt.Fprintf(os.Stderr, "  Run 'devsync update' to upgrade.\n\n")
		}
	}

	os.MkdirAll(filepath.Dir(updateCheckFile), 0700)
	data, _ := json.Marshal(updateCache{Latest: latest, Checked: time.Now()})
	os.WriteFile(updateCheckFile, data, 0600)
}

// NewRoot builds the devsync CLI root command with all subcommands.
func NewRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "devsync",
		Short:         "devsync - end-to-end encrypted credential store",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
				return
			}
			if Version == "dev" {
				return
			}
			checkUpdate()
		},
	}
	root.AddCommand(
		newUpdateCmd(),
		// setup / identity
		newConfigCmd(),
		newInitCmd(),
		newRegisterCmd(),
		newBootstrapAdminCmd(),
		newWhoAmICmd(),
		newUnlockCmd(),
		// team / vault admin
		newCreateTeamCmd(),
		newInviteCmd(),
		newJoinCmd(),
		newTeamsCmd(),
		newMembersCmd(),
		newApproveCmd(),
		newCreateVaultCmd(),
		newGrantCmd(),
		newRevokeCmd(),
		// files
		newPushCmd(),
		newPullCmd(),
		newHistoryCmd(),
		newCheckoutCmd(),
		newRmCmd(),
		newAuditCmd(),
		// device
		newDeviceCmd(),
	)
	return root
}
