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
	companion, err := discoverCompanion(filepath.Dir(executable), config.DataDir)
	if err != nil {
		log.Printf("CLIProxyAPI auto-start is unavailable: %v", err)
	}
	app, err := newApp(config.DataDir, companion)
	if err != nil {
		log.Fatal(err)
	}
	if err := app.prepareAPI(defaultMonitorAddress); err != nil {
		log.Fatal(err)
	}
	defer app.shutdown(context.Background())
	if companion != nil {
		started, startErr := companion.Start()
		if startErr != nil {
			log.Printf("CLIProxyAPI auto-start failed: %v", startErr)
		} else if started {
			log.Printf("CLIProxyAPI started automatically from %s", companion.executable)
		} else {
			log.Printf("CLIProxyAPI is already listening on %s", companionAddress)
		}
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
