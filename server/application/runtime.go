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

	"cpa-monitor/server/internal/accounthealth"
	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/deployments"
	"cpa-monitor/server/internal/gateways"
	cpagateway "cpa-monitor/server/internal/gateways/cpa"
	"cpa-monitor/server/internal/gateways/sub2api"
	"cpa-monitor/server/internal/httpapi"
	"cpa-monitor/server/internal/model"
	"cpa-monitor/server/internal/observability"
	"cpa-monitor/server/internal/scraper"
	"cpa-monitor/server/internal/subscriptions"
)

// Runtime owns the reusable CPA Orbit backend used by both the HTTP server and
// the desktop application.
type Runtime struct {
	root        string
	handler     http.Handler
	server      *httpapi.Server
	monitor     *httpapi.Monitor
	settings    *config.Store
	control     *controlplane.Store
	deployments *deployments.Coordinator
	collector   *observability.Collector
	accountPoll *accounthealth.Scheduler
	startOnce   sync.Once
	closeOnce   sync.Once
	closeErr    error
}

// Settings and Alert are public aliases so the desktop host can consume the
// shared runtime without reaching into the server's internal packages.
type Settings = config.Settings
type Alert = model.Alert

// New creates a backend rooted at root. Mutable data is stored below root/data
// and subscription archives below root/subscriptions/{sub2api,cpa}/MMDD.
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
	if err := migrateLegacySubscriptionRoot(absoluteRoot); err != nil {
		return nil, err
	}

	settings, err := config.NewStore(filepath.Join(dataDir, "settings.json"))
	if err != nil {
		return nil, err
	}
	control, err := controlplane.NewStore(filepath.Join(dataDir, "control-plane.db"))
	if err != nil {
		return nil, err
	}
	subs, err := subscriptions.NewManager(
		filepath.Join(absoluteRoot, "subscriptions"),
		filepath.Join(dataDir, "subscription_checks.json"),
		settings,
	)
	if err != nil {
		_ = control.Close()
		return nil, err
	}
	if targets, listErr := control.ListGatewayTargets(context.Background()); listErr != nil {
		_ = control.Close()
		return nil, listErr
	} else if len(targets) == 0 {
		current := settings.Get()
		if _, seedErr := control.UpsertGatewayTarget(context.Background(), controlplane.GatewayTarget{
			Kind: string(gateways.KindCPA), Name: "Legacy CPA", BaseURL: current.BaseURL,
			AdminKey: current.CPAManagementKey, Enabled: true, Primary: true, DefaultConcurrency: 1, RateMultiplier: 1,
		}); seedErr != nil {
			_ = control.Close()
			return nil, seedErr
		}
	}
	coordinator := deployments.NewCoordinator(control, subs, func(target controlplane.GatewayTarget, secret string) (gateways.Adapter, error) {
		switch gateways.Kind(target.Kind) {
		case gateways.KindCPA:
			return cpagateway.NewClient(func() cpagateway.Config {
				current := settings.Get()
				return cpagateway.Config{BaseURL: target.BaseURL, ManagementKey: secret, AuthDir: current.CPAAuthDir, SyncEnabled: current.SyncToCPAAuthDir}
			}, filepath.Join(dataDir, fmt.Sprintf("cpa-managed-files-%d.json", target.ID))), nil
		case gateways.KindSub2API:
			return sub2api.NewClient(func() sub2api.Config { return sub2api.Config{BaseURL: target.BaseURL, AdminKey: secret} }), nil
		default:
			return nil, fmt.Errorf("unsupported gateway kind %q", target.Kind)
		}
	})
	collector := observability.NewCollector(control, func(target controlplane.GatewayTarget, secret string) (observability.Source, error) {
		if gateways.Kind(target.Kind) != gateways.KindSub2API {
			return nil, fmt.Errorf("target %d is not Sub2API", target.ID)
		}
		return sub2api.NewClient(func() sub2api.Config { return sub2api.Config{BaseURL: target.BaseURL, AdminKey: secret} }), nil
	})
	monitor, err := httpapi.NewMonitor(
		filepath.Join(dataDir, "offers.json"),
		filepath.Join(dataDir, "alerts.json"),
		settings,
		scraper.NewClient(),
	)
	if err != nil {
		_ = control.Close()
		return nil, err
	}

	accountPoll := accounthealth.NewScheduler(settings, coordinator, subs)
	server := httpapi.NewServer(settings, monitor, subs, control, coordinator, collector)
	server.SetAccountPoller(accountPoll)
	return &Runtime{
		root: absoluteRoot, handler: server.Handler(), server: server,
		monitor: monitor, settings: settings, control: control, deployments: coordinator, collector: collector, accountPoll: accountPoll,
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
		r.collector.Start(ctx)
		r.accountPoll.Start(ctx)
	})
}

// Close releases durable local control-plane resources. It is safe to call
// repeatedly from overlapping native and process shutdown hooks.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		if r.control != nil {
			r.closeErr = r.control.Close()
		}
	})
	return r.closeErr
}
