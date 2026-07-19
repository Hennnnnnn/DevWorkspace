package main

import (
	"fmt"
	"os"

	"golang.org/x/term"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/commands"
	"github.com/Hennnnnnn/DevWorkspace/internal/client/tui"
)

func main() {
	if len(os.Args) == 1 && term.IsTerminal(int(os.Stdout.Fd())) {
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if err := commands.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
