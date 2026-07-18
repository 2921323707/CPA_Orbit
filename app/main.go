package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// Desktop builds stage the existing Vue application into this directory.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	config, err := loadDesktopConfig()
	if err != nil {
		log.Fatal(err)
	}
	app, err := newApp(config.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("CPA Orbit desktop data directory: %s", config.DataDir)
	if config.ConfigPath != "" {
		log.Printf("CPA Orbit desktop config: %s", config.ConfigPath)
	}

	err = wails.Run(&options.App{
		Title:                    "CPA Orbit",
		Width:                    config.WindowWidth,
		Height:                   config.WindowHeight,
		MinWidth:                 960,
		MinHeight:                640,
		BackgroundColour:         options.NewRGB(16, 36, 43),
		AssetServer:              &assetserver.Options{Assets: assets, Handler: app.handler()},
		OnStartup:                app.startup,
		OnShutdown:               app.shutdown,
		EnableDefaultContextMenu: false,
	})
	if err != nil {
		log.Fatal(err)
	}
}
