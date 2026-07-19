package main

import (
	"fmt"
	"os"

	"github.com/Hennnnnnn/DevWorkspace/internal/client/commands"
)

func main() {
	if err := commands.NewRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
