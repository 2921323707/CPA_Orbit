package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"cpa-monitor/server/application"
)

const defaultMonitorAddress = "127.0.0.1:8080"

// App hosts the reusable backend inside the native desktop lifecycle.
type App struct {
	runtime        *application.Runtime
	companion      *companionService
	assetHandler   http.Handler
	apiServer      *http.Server
	apiAddress     string
	ownsAPIRuntime bool
	cancel         context.CancelFunc
	tray           *trayController
	ctx            context.Context
	quitting       atomic.Bool
	settingsMu     sync.Mutex
	lastSettings   application.Settings
	settingsReady  bool
}

func newApp(dataDir string, companion *companionService) (*App, error) {
	appRuntime, err := application.New(dataDir)
	if err != nil {
		return nil, err
	}
	app := &App{runtime: appRuntime, companion: companion}
	appRuntime.SetSettingsUpdateHandler(app.settingsUpdated)
	appRuntime.SetAlertHandler(app.handleAlert)
	return app, nil
}

func (a *App) handler() http.Handler {
	return a.assetHandler
}

// prepareAPI gives the desktop and browser frontends one live backend. It
// owns port 8080 when available, otherwise it reuses an already-running CPA
// Orbit Monitor API and proxies desktop /api requests to that process.
func (a *App) prepareAPI(address string) error {
	listener, listenErr := net.Listen("tcp", address)
	if listenErr == nil {
		a.assetHandler = a.runtime.DesktopHandler()
		a.ownsAPIRuntime = true
		server := &http.Server{
			Handler:           a.runtime.Handler(),
			ReadHeaderTimeout: 5 * time.Second,
		}
		a.apiServer = server
		a.apiAddress = listener.Addr().String()
		go func() {
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Printf("Monitor API stopped: %v", err)
			}
		}()
		log.Printf("Monitor API listening on http://%s", listener.Addr())
		return nil
	}

	baseURL := "http://" + address
	if !monitorAPIHealthy(baseURL) {
		return fmt.Errorf("listen on %s: %w; the existing listener is not CPA Orbit", address, listenErr)
	}
	target, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.Header.Del("Origin")
	}
	a.assetHandler = http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api" && !strings.HasPrefix(request.URL.Path, "/api/") {
			http.NotFound(response, request)
			return
		}
		proxy.ServeHTTP(response, request)
	})
	a.apiAddress = address
	log.Printf("Reusing Monitor API at %s", baseURL)
	return nil
}

func monitorAPIHealthy(baseURL string) bool {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	response, err := client.Get(strings.TrimRight(baseURL, "/") + "/api/health")
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}
	var payload struct {
		Status string `json:"status"`
		Name   string `json:"name"`
	}
	return json.NewDecoder(response.Body).Decode(&payload) == nil && payload.Status == "ok" && payload.Name == "CPA Orbit"
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.initializeNativeFeatures(ctx)
	go a.watchSettings(ctx)

	if !a.ownsAPIRuntime {
		return
	}
	workerContext, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	a.runtime.Start(workerContext)
}

func (a *App) shutdown(context.Context) {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	if err := a.companion.Stop(); err != nil {
		log.Printf("stop CLIProxyAPI: %v", err)
	}
	if a.apiServer != nil {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.apiServer.Shutdown(shutdownContext); err != nil {
			log.Printf("stop Monitor API: %v", err)
		}
		a.apiServer = nil
	}
	if a.tray != nil {
		a.tray.Stop()
		a.tray = nil
	}
}
