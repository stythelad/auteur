package main

import (
	"fmt"
	"os"

	"github.com/pokhrelmishan/sentinel/cmd/sentinel/commands"
)

func main() {
	if err := commands.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
