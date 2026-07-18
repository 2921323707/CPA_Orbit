package main

import (
	"context"
	"net/http"

	"cpa-monitor/server/application"
)

// App hosts the reusable backend inside the native desktop lifecycle.
type App struct {
	runtime *application.Runtime
	cancel  context.CancelFunc
}

func newApp(dataDir string) (*App, error) {
	appRuntime, err := application.New(dataDir)
	if err != nil {
		return nil, err
	}
	return &App{runtime: appRuntime}, nil
}

func (a *App) handler() http.Handler {
	return a.runtime.DesktopHandler()
}

func (a *App) startup(ctx context.Context) {
	workerContext, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.runtime.Start(workerContext)
}

func (a *App) shutdown(context.Context) {
	if a.cancel != nil {
		a.cancel()
	}
}
