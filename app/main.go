package main

import (
	"context"
	"embed"
	"log"
	"os"
	"path/filepath"

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
	executable, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	companions, err := discoverCompanions(filepath.Dir(executable), config.DataDir, config)
	if err != nil {
		log.Printf("desktop companion discovery warning: %v", err)
	}
	app, err := newApp(config.DataDir, companions)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.prepareAPI(defaultMonitorAddress); err != nil {
		log.Fatal(err)
	}
	defer app.shutdown(context.Background())
	if err := companions.Start(); err != nil {
		log.Printf("desktop companion startup failed: %v", err)
	}
	log.Printf("CPA Orbit desktop data directory: %s", config.DataDir)
	if config.ConfigPath != "" {
		log.Printf("CPA Orbit desktop config: %s", config.ConfigPath)
	}

	err = wails.Run(&options.App{
		Title:                    "CPA Orbit",
		Width:                    config.WindowWidth,
		Height:                   config.WindowHeight,
		MinWidth:                 minimumWindowWidth,
		MinHeight:                minimumWindowHeight,
		DisableResize:            true,
		BackgroundColour:         options.NewRGB(16, 36, 43),
		AssetServer:              &assetserver.Options{Assets: assets, Handler: app.handler()},
		OnStartup:                app.startup,
		OnBeforeClose:            app.beforeClose,
		OnShutdown:               app.shutdown,
		EnableDefaultContextMenu: false,
	})
	if err != nil {
		log.Printf("CPA Orbit stopped with an error: %v", err)
	}
}
