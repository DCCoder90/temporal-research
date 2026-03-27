//go:build nogui

package main

import (
	"os"
	"temporal-analyze/cli"
)

func main() {
	cli.Run(os.Args[1:])
}
