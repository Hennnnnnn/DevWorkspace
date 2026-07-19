package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/devsync/devsync/internal/client/agent"
	"github.com/devsync/devsync/internal/client/api"
	"github.com/devsync/devsync/internal/client/config"
)

// authedClient loads config + the unlocked keypair from the agent and returns
// a signed API client.
func authedClient() (*api.Client, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	kp, err := agent.Get()
	if err != nil {
		return nil, nil, err
	}
	cl, err := api.New(cfg, kp)
	return cl, cfg, err
}

// promptPassphrase reads a passphrase without echo. For non-interactive use
// (scripts, CI) it falls back to the DEVSYNC_PASSPHRASE env var.
func promptPassphrase(label string) (string, error) {
	if p := os.Getenv("DEVSYNC_PASSPHRASE"); p != "" {
		return p, nil
	}
	fmt.Fprint(os.Stderr, label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// promptLine reads a line of plain input.
func promptLine(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	s, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(s), err
}

// expectArgs returns a cobra positional-arg validator that shows the full
// usage line (e.g. "devsync create-vault <name> --team <team>") when the
// user passes the wrong number of arguments, instead of cobra's terse
// "accepts 1 arg(s), received 0".
func expectArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return fmt.Errorf("usage: %s %s", cmd.CommandPath(), strings.TrimPrefix(cmd.Use, cmd.Name()+" "))
		}
		return nil
	}
}

// maxArgs is the same as expectArgs but for "at most N" constraints.
func maxArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return fmt.Errorf("usage: %s %s", cmd.CommandPath(), strings.TrimPrefix(cmd.Use, cmd.Name()+" "))
		}
		return nil
	}
}
