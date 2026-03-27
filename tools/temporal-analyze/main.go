//go:build !nogui

package main

import (
	"embed"
	"fmt"
	"os"
	"strings"
	"temporal-analyze/cli"
	"temporal-analyze/internal/config"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	// Only enter CLI mode for user-supplied arguments.
	// Wails passes its own internal flags (e.g. -wails-ensure-runtime) during
	// binding generation — those must reach wails.Run(), not our CLI handler.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-wails") {
		if err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "Configuration error: %s\n", err)
			os.Exit(1)
		}
		cli.Run(os.Args[1:])
		return
	}

	app := NewApp()
	err := wails.Run(&options.App{
		Title:  "Temporal Analyze",
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:   app.startup,
		Bind:        []interface{}{app},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
	})
	if err != nil {
		os.Exit(1)
	}
}
