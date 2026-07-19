package application

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/httpapi"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/scraper"
	"cpa-monitor/server/internal/subscriptions"
)

// Runtime owns the reusable CPA Orbit backend used by both the HTTP server and
// the desktop application.
type Runtime struct {
	root      string
	handler   http.Handler
	server    *httpapi.Server
	monitor   *httpapi.Monitor
	settings  *config.Store
	startOnce sync.Once
}

// Settings and Alert are public aliases so the desktop host can consume the
// shared runtime without reaching into the server's internal packages.
type Settings = config.Settings
type Alert = model.Alert

// New creates a backend rooted at root. Mutable data is stored below root/data
// and subscription archives below root/k12.
func New(root string) (*Runtime, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("application root is required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve application root: %w", err)
	}
	if err := os.MkdirAll(absoluteRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create application root: %w", err)
	}
	dataDir := filepath.Join(absoluteRoot, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	settings, err := config.NewStore(filepath.Join(dataDir, "settings.json"))
	if err != nil {
		return nil, err
	}
	subs, err := subscriptions.NewManager(
		filepath.Join(absoluteRoot, "k12"),
		filepath.Join(dataDir, "subscription_checks.json"),
		settings,
	)
	if err != nil {
		return nil, err
	}
	monitor, err := httpapi.NewMonitor(
		filepath.Join(dataDir, "offers.json"),
		filepath.Join(dataDir, "alerts.json"),
		settings,
		scraper.NewClient(),
	)
	if err != nil {
		return nil, err
	}

	server := httpapi.NewServer(settings, monitor, subs)
	return &Runtime{
		root: absoluteRoot, handler: server.Handler(), server: server,
		monitor: monitor, settings: settings,
	}, nil
}

func (r *Runtime) SetSettingsUpdateHandler(handler func(config.Settings)) {
	r.server.SetSettingsUpdateHandler(handler)
}

func (r *Runtime) Settings() config.Settings {
	return r.settings.Get()
}

// ReloadSettings picks up changes made by another frontend sharing the same
// data directory (for example the browser while the desktop app is running).
func (r *Runtime) ReloadSettings() error {
	return r.settings.Reload()
}

func (r *Runtime) SetAlertHandler(handler func(model.Alert)) {
	r.monitor.SetAlertHandler(handler)
}

// Handler returns the HTTP API with its normal network-facing CORS policy.
func (r *Runtime) Handler() http.Handler {
	return r.handler
}

// DesktopHandler returns the same API for Wails' in-process asset server. The
// WebView origin is trusted by the native host, so it is removed before the
// network-facing CORS middleware runs.
func (r *Runtime) DesktopHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api" && !strings.HasPrefix(request.URL.Path, "/api/") {
			http.NotFound(w, request)
			return
		}
		desktopRequest := request.Clone(request.Context())
		desktopRequest.Header = request.Header.Clone()
		desktopRequest.Header.Del("Origin")
		r.handler.ServeHTTP(w, desktopRequest)
	})
}

// Root returns the absolute mutable-data root.
func (r *Runtime) Root() string {
	return r.root
}

// Start launches background refresh work until ctx is cancelled.
func (r *Runtime) Start(ctx context.Context) {
	r.startOnce.Do(func() {
		r.monitor.Start(ctx)
	})
}
