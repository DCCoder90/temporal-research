//go:build !nogui

package main

import (
	"embed"
	"os"
	"temporal-analyze/cli"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	if len(os.Args) > 1 {
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
