package deployments

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/gateways"
	"cpa-monitor/server/internal/model"
)

type fakeSource struct{ credential gateways.Credential }

func (f fakeSource) Get(id string) (model.Subscription, bool) {
	return model.Subscription{ID: id, Email: f.credential.Email}, true
}
func (f fakeSource) GatewayCredential(id string) (gateways.Credential, error) {
	credential := f.credential
	credential.SubscriptionID = id
	return credential, nil
}

type fakeAdapter struct {
	kind       gateways.Kind
	deployErr  error
	deployID   string
	deploys    int
	detaches   int
	lastDetach gateways.BindingRef
}

func (f *fakeAdapter) Kind() gateways.Kind { return f.kind }
func (f *fakeAdapter) Health(context.Context) (gateways.Health, error) {
	return gateways.Health{Status: "ok"}, nil
}
func (f *fakeAdapter) Deploy(context.Context, gateways.Credential, gateways.DeployOptions) (gateways.DeploymentResult, error) {
	f.deploys++
	if f.deployErr != nil {
		return gateways.DeploymentResult{}, f.deployErr
	}
	return gateways.DeploymentResult{Binding: gateways.BindingRef{ExternalID: f.deployID, Managed: true}, Status: "deployed"}, nil
}
func (f *fakeAdapter) Inspect(context.Context, gateways.BindingRef, gateways.Credential) (gateways.InspectResult, error) {
	return gateways.InspectResult{}, nil
}
func (f *fakeAdapter) Detach(_ context.Context, binding gateways.BindingRef, _ gateways.Credential) error {
	f.detaches++
	f.lastDetach = binding
	return nil
}

func TestDeployDefaultUsesSub2APIPrimary(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	primary, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "sub2api", Name: "primary", BaseURL: "http://127.0.0.1:8080", Enabled: true, Primary: true, DefaultGroupIDs: []int64{3}, DefaultConcurrency: 2})
	adapter := &fakeAdapter{kind: gateways.KindSub2API, deployID: "42"}
	coordinator := NewCoordinator(store, fakeSource{credential: gateways.Credential{Email: "plus@example.com", Data: []byte(`{"access_token":"token"}`)}}, func(controlplane.GatewayTarget, string) (gateways.Adapter, error) { return adapter, nil })

	result, err := coordinator.DeployDefault(ctx, "sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetID != primary.ID || result.RemoteAccountID != "42" || result.ObservedState != "active" || adapter.deploys != 1 {
		t.Fatalf("unexpected binding: %+v deploys=%d", result, adapter.deploys)
	}
	if _, err := coordinator.Deploy(ctx, "sub-1", primary.ID); err != nil {
		t.Fatal(err)
	}
	if adapter.deploys != 2 {
		t.Fatalf("duplicate deployment was not idempotently updated: %d", adapter.deploys)
	}
}

func TestDeployDefaultFallsBackToCPA(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	primary, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "sub2api", Name: "primary", BaseURL: "http://127.0.0.1:8080", Enabled: true, Primary: true})
	fallback, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "cpa", Name: "fallback", BaseURL: "http://127.0.0.1:8317/v1", Enabled: true})
	// Reassert Sub2API after creating the fallback.
	primary.Primary = true
	primary, _ = store.UpsertGatewayTarget(ctx, primary)
	primaryAdapter := &fakeAdapter{kind: gateways.KindSub2API, deployErr: errors.New("offline")}
	fallbackAdapter := &fakeAdapter{kind: gateways.KindCPA, deployID: "owned.json"}
	coordinator := NewCoordinator(store, fakeSource{credential: gateways.Credential{Data: []byte(`{"access_token":"token","email":"x@example.com"}`)}}, func(target controlplane.GatewayTarget, _ string) (gateways.Adapter, error) {
		if target.ID == fallback.ID {
			return fallbackAdapter, nil
		}
		return primaryAdapter, nil
	})
	result, err := coordinator.DeployDefault(ctx, "sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetID != fallback.ID || result.Mode != "fallback" || fallbackAdapter.deploys != 1 {
		t.Fatalf("fallback was not used: %+v", result)
	}
}

func TestDeployRequiresMigrationWhenAnotherGatewayIsActive(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	primary, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "sub2api", Name: "primary", BaseURL: "http://127.0.0.1:8080", Enabled: true, Primary: true})
	fallback, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "cpa", Name: "fallback", BaseURL: "http://127.0.0.1:8317/v1", Enabled: true})
	_, _ = store.UpsertDeploymentBinding(ctx, controlplane.DeploymentBinding{SubscriptionID: "sub-1", TargetID: fallback.ID, RemoteAccountID: "owned.json", Ownership: "managed", DesiredState: "active", ObservedState: "active", Mode: "fallback"})
	adapter := &fakeAdapter{kind: gateways.KindSub2API, deployID: "42"}
	coordinator := NewCoordinator(store, fakeSource{credential: gateways.Credential{Data: []byte(`{}`)}}, func(controlplane.GatewayTarget, string) (gateways.Adapter, error) { return adapter, nil })
	if _, err := coordinator.Deploy(ctx, "sub-1", primary.ID); !errors.Is(err, ErrActiveBindingExists) {
		t.Fatalf("expected migration guard, got %v", err)
	}
	if adapter.deploys != 0 {
		t.Fatal("destination was deployed before source detachment")
	}
}

func TestDetachDoesNotDeleteAdoptedRemoteAccount(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	target, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "sub2api", Name: "primary", BaseURL: "http://127.0.0.1:8080", Enabled: true, Primary: true})
	_, _ = store.UpsertDeploymentBinding(ctx, controlplane.DeploymentBinding{SubscriptionID: "sub-1", TargetID: target.ID, RemoteAccountID: "99", Ownership: "adopted", DesiredState: "active", ObservedState: "active"})
	adapter := &fakeAdapter{kind: gateways.KindSub2API}
	coordinator := NewCoordinator(store, fakeSource{credential: gateways.Credential{Data: []byte(`{}`)}}, func(controlplane.GatewayTarget, string) (gateways.Adapter, error) { return adapter, nil })
	if _, err := coordinator.Detach(ctx, "sub-1", target.ID); err != nil {
		t.Fatal(err)
	}
	if adapter.detaches != 1 || adapter.lastDetach.Managed {
		t.Fatalf("adopted account received managed delete: %+v", adapter.lastDetach)
	}
}

func TestDetachAllCleansManagedAndOnlyUnbindsAdoptedAccounts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	primary, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "sub2api", Name: "primary", BaseURL: "http://127.0.0.1:8080", Enabled: true, Primary: true})
	fallback, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "cpa", Name: "fallback", BaseURL: "http://127.0.0.1:8317/v1", Enabled: true})
	failed, _ := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "sub2api", Name: "failed", BaseURL: "http://127.0.0.1:8081", Enabled: true})
	_, _ = store.UpsertDeploymentBinding(ctx, controlplane.DeploymentBinding{SubscriptionID: "sub-1", TargetID: primary.ID, RemoteAccountID: "42", Ownership: "managed", DesiredState: "active", ObservedState: "active"})
	_, _ = store.UpsertDeploymentBinding(ctx, controlplane.DeploymentBinding{SubscriptionID: "sub-1", TargetID: fallback.ID, RemoteAccountID: "adopted.json", Ownership: "adopted", DesiredState: "active", ObservedState: "active"})
	_, _ = store.UpsertDeploymentBinding(ctx, controlplane.DeploymentBinding{SubscriptionID: "sub-1", TargetID: failed.ID, Ownership: "managed", DesiredState: "active", ObservedState: "error", LastError: "import failed"})
	adapters := map[int64]*fakeAdapter{primary.ID: {kind: gateways.KindSub2API}, fallback.ID: {kind: gateways.KindCPA}, failed.ID: {kind: gateways.KindSub2API}}
	coordinator := NewCoordinator(store, fakeSource{credential: gateways.Credential{Data: []byte(`{}`)}}, func(target controlplane.GatewayTarget, _ string) (gateways.Adapter, error) {
		return adapters[target.ID], nil
	})
	if err := coordinator.DetachAll(ctx, "sub-1"); err != nil {
		t.Fatal(err)
	}
	if adapters[primary.ID].detaches != 1 || !adapters[primary.ID].lastDetach.Managed {
		t.Fatalf("managed binding was not deleted: %+v", adapters[primary.ID])
	}
	if adapters[fallback.ID].detaches != 1 || adapters[fallback.ID].lastDetach.Managed {
		t.Fatalf("adopted binding was treated as owned: %+v", adapters[fallback.ID])
	}
	if adapters[failed.ID].detaches != 0 {
		t.Fatal("failed deployment without a remote ID attempted a remote delete")
	}
	bindings, _ := store.ListDeploymentBindings(ctx, "sub-1")
	for _, binding := range bindings {
		if binding.ObservedState != "detached" || binding.DesiredState != "detached" {
			t.Fatalf("binding remains active: %+v", binding)
		}
	}
}

func TestValidateTargetRequiresHTTPSForRemote(t *testing.T) {
	if err := ValidateTarget(controlplane.GatewayTarget{Kind: "sub2api", BaseURL: "http://gateway.example.com", AllowRemote: true}); err == nil {
		t.Fatal("expected remote HTTP target rejection")
	}
	if err := ValidateTarget(controlplane.GatewayTarget{Kind: "sub2api", BaseURL: "https://gateway.example.com", AllowRemote: true}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTarget(controlplane.GatewayTarget{Kind: "sub2api", BaseURL: "http://127.0.0.1:8080"}); err != nil {
		t.Fatal(err)
	}
}

func newTestStore(t *testing.T) *controlplane.Store {
	t.Helper()
	store, err := controlplane.NewStore(filepath.Join(t.TempDir(), "control.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
