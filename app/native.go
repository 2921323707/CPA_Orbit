package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"time"

	"cpa-monitor/server/application"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) initializeNativeFeatures(ctx context.Context) {
	a.settingsMu.Lock()
	a.ctx = ctx
	a.settingsMu.Unlock()
	a.applySettings(a.runtime.Settings())
	a.tray = newTray(a.showWindow, a.quitFromTray)
	a.tray.Start()
	if err := wailsruntime.InitializeNotifications(ctx); err != nil {
		log.Printf("desktop notifications unavailable: %v", err)
	}
}

func (a *App) settingsUpdated(settings application.Settings) {
	a.applySettings(settings)
}

// watchSettings keeps native behavior in sync when the browser frontend is
// connected to the already-running shared API process.
func (a *App) watchSettings(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.runtime.ReloadSettings(); err != nil {
				continue
			}
			a.applySettings(a.runtime.Settings())
		}
	}
}

func (a *App) applySettings(settings application.Settings) {
	a.settingsMu.Lock()
	if a.settingsReady && reflect.DeepEqual(a.lastSettings, settings) {
		a.settingsMu.Unlock()
		return
	}
	a.lastSettings = settings
	a.settingsReady = true
	a.settingsMu.Unlock()

	executable, err := os.Executable()
	if err != nil {
		return
	}
	if err := setStartOnLogin(executable, settings.StartOnLogin); err != nil {
		log.Printf("update startup setting: %v", err)
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	a.settingsMu.Lock()
	settings := a.lastSettings
	a.settingsMu.Unlock()
	if !a.quitting.Load() && settings.CloseToTray && a.tray != nil {
		wailsruntime.WindowHide(ctx)
		// Wails interprets a true return value as "prevent close".
		return true
	}
	return false
}

func (a *App) showWindow() {
	a.settingsMu.Lock()
	ctx := a.ctx
	a.settingsMu.Unlock()
	if ctx == nil {
		return
	}
	wailsruntime.WindowShow(ctx)
	wailsruntime.WindowUnminimise(ctx)
}

func (a *App) quitFromTray() {
	a.quitting.Store(true)
	a.settingsMu.Lock()
	ctx := a.ctx
	a.settingsMu.Unlock()
	if ctx != nil {
		wailsruntime.Quit(ctx)
	}
}

func (a *App) handleAlert(alert application.Alert) {
	a.settingsMu.Lock()
	settings := a.lastSettings
	ctx := a.ctx
	a.settingsMu.Unlock()
	if ctx == nil {
		return
	}
	source := "K12"
	if alert.Source == "gpt-plus" {
		source = "GPT Plus"
	}
	body := fmt.Sprintf("[%s] %s 当前 ¥%.2f，阈值 ¥%.2f", source, alert.Title, alert.Price, alert.Threshold)
	if settings.FlashOnAlert {
		flashWindow("CPA Orbit")
	}
	if !settings.DesktopNotifications {
		return
	}
	if err := wailsruntime.SendNotification(ctx, wailsruntime.NotificationOptions{
		ID: alert.ID, Title: "CPA Orbit · 低价提醒", Body: body,
	}); err != nil {
		log.Printf("send desktop notification: %v", err)
	}
}
