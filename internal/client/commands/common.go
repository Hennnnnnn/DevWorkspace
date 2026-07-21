package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/actions"
)

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

// isTTY reports whether stdin is an interactive terminal.
func isTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// pickFromList shows a numbered list and asks the user to choose one item.
// Caller must ensure stdin is a TTY.
func pickFromList(label string, items []string) (string, error) {
	fmt.Fprintf(os.Stderr, "Multiple %ss:\n", label)
	for i, it := range items {
		fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, it)
	}
	s, err := promptLine(fmt.Sprintf("Choose %s [1-%d]: ", label, len(items)))
	if err != nil {
		return "", err
	}
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil || n < 1 || n > len(items) {
		return "", fmt.Errorf("invalid choice %q", s)
	}
	return items[n-1], nil
}

// resolveVault resolves the vault to operate on: the flag value if given,
// the only accessible vault if there is exactly one, otherwise an
// interactive picker (TTY) or a clear error listing the options (non-TTY).
func resolveVault(vault string) (string, error) {
	if vault != "" {
		return vault, nil
	}
	vaults, err := actions.ListVaults()
	if err != nil {
		return "", err
	}
	names := make([]string, len(vaults))
	for i, v := range vaults {
		names[i] = v.Name
	}
	switch len(names) {
	case 0:
		return "", fmt.Errorf("no vaults accessible")
	case 1:
		return names[0], nil
	}
	if !isTTY() {
		return "", fmt.Errorf("multiple vaults, pass --vault <name>: %s", strings.Join(names, ", "))
	}
	return pickFromList("vault", names)
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
