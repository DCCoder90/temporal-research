//go:build nogui

package main

import (
	"fmt"
	"os"
	"temporal-analyze/cli"
	"temporal-analyze/internal/config"
)

func main() {
	if err := config.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
		os.Exit(1)
	}
	cli.Run(os.Args[1:])
}
