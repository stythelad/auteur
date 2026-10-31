package main

import (
	"fmt"
	"os"

	"github.com/stythelad/auteur/cmd/auteur/commands"
)

func main() {
	if err := commands.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
