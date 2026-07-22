package deployments

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/gateways"
	"cpa-monitor/server/internal/model"
)

var ErrActiveBindingExists = errors.New("subscription already has an active gateway binding; migrate it instead")

type SubscriptionSource interface {
	Get(string) (model.Subscription, bool)
	GatewayCredential(string) (gateways.Credential, error)
}

type AdapterFactory func(controlplane.GatewayTarget, string) (gateways.Adapter, error)

type Coordinator struct {
	store   *controlplane.Store
	source  SubscriptionSource
	factory AdapterFactory
	mu      sync.Mutex
}

func NewCoordinator(store *controlplane.Store, source SubscriptionSource, factory AdapterFactory) *Coordinator {
	return &Coordinator{store: store, source: source, factory: factory}
}

func (c *Coordinator) UpsertTarget(ctx context.Context, target controlplane.GatewayTarget) (controlplane.GatewayTarget, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ValidateTarget(target); err != nil {
		return controlplane.GatewayTarget{}, err
	}
	return c.store.UpsertGatewayTarget(ctx, target)
}

func (c *Coordinator) Targets(ctx context.Context) ([]controlplane.GatewayTarget, error) {
	return c.store.ListGatewayTargets(ctx)
}

func (c *Coordinator) TargetHealth(ctx context.Context, targetID int64) (gateways.Health, error) {
	target, err := c.store.GatewayTarget(ctx, targetID)
	if err != nil {
		return gateways.Health{}, err
	}
	if err := ValidateTarget(target); err != nil {
		return gateways.Health{}, err
	}
	secret, err := c.store.GatewayTargetSecret(ctx, targetID)
	if err != nil {
		return gateways.Health{}, err
	}
	adapter, err := c.factory(target, secret)
	if err != nil {
		return gateways.Health{}, err
	}
	return adapter.Health(ctx)
}

func ValidateTarget(target controlplane.GatewayTarget) error {
	kind := gateways.Kind(strings.ToLower(strings.TrimSpace(target.Kind)))
	if kind != gateways.KindCPA && kind != gateways.KindSub2API {
		return errors.New("gateway kind must be cpa or sub2api")
	}
	parsed, err := url.ParseRequestURI(strings.TrimSpace(target.BaseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil {
		return errors.New("gateway base URL must be an absolute HTTP(S) URL without credentials")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("gateway base URL must use HTTP or HTTPS")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	loopback := host == "localhost"
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		loopback = true
	}
	if !loopback {
		if !target.AllowRemote {
			return errors.New("remote gateway targets require explicit allowRemote")
		}
		if parsed.Scheme != "https" {
			return errors.New("remote gateway targets must use HTTPS")
		}
	}
	if target.DefaultConcurrency < 0 || target.DefaultConcurrency > 1000 {
		return errors.New("default concurrency must be between 0 and 1000")
	}
	if target.DefaultPriority < -1000 || target.DefaultPriority > 1000 {
		return errors.New("default priority must be between -1000 and 1000")
	}
	if target.RateMultiplier < 0 || target.RateMultiplier > 1000 {
		return errors.New("rate multiplier must be between 0 and 1000")
	}
	for _, groupID := range target.DefaultGroupIDs {
		if groupID <= 0 {
			return errors.New("default group IDs must be positive")
		}
	}
	return nil
}

func (c *Coordinator) DeployDefault(ctx context.Context, subscriptionID string) (controlplane.DeploymentBinding, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deployDefault(ctx, subscriptionID)
}

func (c *Coordinator) deployDefault(ctx context.Context, subscriptionID string) (controlplane.DeploymentBinding, error) {
	targets, err := c.store.ListGatewayTargets(ctx)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	credential, err := c.source.GatewayCredential(subscriptionID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	if gateways.IsSub2APIDataPackage(credential.Data) {
		var selected *controlplane.GatewayTarget
		for i := range targets {
			if !targets[i].Enabled || targets[i].Kind != string(gateways.KindSub2API) {
				continue
			}
			if selected == nil || targets[i].Primary {
				selected = &targets[i]
			}
			if targets[i].Primary {
				break
			}
		}
		if selected == nil {
			return controlplane.DeploymentBinding{}, errors.New("no enabled Sub2API gateway target is configured")
		}
		return c.deploy(ctx, subscriptionID, selected.ID)
	}
	var primary *controlplane.GatewayTarget
	for i := range targets {
		if targets[i].Enabled && targets[i].Primary {
			primary = &targets[i]
			break
		}
	}
	if primary == nil {
		return controlplane.DeploymentBinding{}, errors.New("no enabled primary gateway target is configured")
	}
	binding, primaryErr := c.deploy(ctx, subscriptionID, primary.ID)
	if primaryErr == nil {
		return binding, nil
	}
	for _, target := range targets {
		if !target.Enabled || target.Primary || target.Kind != string(gateways.KindCPA) {
			continue
		}
		binding, fallbackErr := c.deploy(ctx, subscriptionID, target.ID)
		if fallbackErr == nil {
			return binding, nil
		}
	}
	return controlplane.DeploymentBinding{}, fmt.Errorf("primary gateway deployment failed and CPA fallback was unavailable: %w", primaryErr)
}

func (c *Coordinator) Deploy(ctx context.Context, subscriptionID string, targetID int64) (controlplane.DeploymentBinding, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.deploy(ctx, subscriptionID, targetID)
}

func (c *Coordinator) deploy(ctx context.Context, subscriptionID string, targetID int64) (controlplane.DeploymentBinding, error) {
	target, err := c.store.GatewayTarget(ctx, targetID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	if !target.Enabled {
		return controlplane.DeploymentBinding{}, errors.New("gateway target is disabled")
	}
	bindings, listErr := c.store.ListDeploymentBindings(ctx, subscriptionID)
	if listErr != nil {
		return controlplane.DeploymentBinding{}, listErr
	}
	for _, binding := range bindings {
		if binding.TargetID != targetID && binding.DesiredState == "active" && binding.ObservedState == "active" {
			return controlplane.DeploymentBinding{}, ErrActiveBindingExists
		}
	}
	credential, err := c.source.GatewayCredential(subscriptionID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	secret, err := c.store.GatewayTargetSecret(ctx, targetID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	adapter, err := c.factory(target, secret)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	operation, err := c.store.CreateSyncOperation(ctx, controlplane.SyncOperation{SubscriptionID: subscriptionID, TargetID: targetID, Kind: "deploy", Status: "running", Attempt: 1})
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	mode := "fallback"
	if target.Primary {
		mode = "primary"
	}
	if strings.TrimSpace(credential.LogicalIdentity) != "" {
		if err := c.store.ReserveCredentialAssignment(ctx, credential.LogicalIdentity, subscriptionID, targetID); err != nil {
			_ = c.store.CompleteSyncOperation(ctx, operation.ID, "failed", cleanError(err.Error()))
			return controlplane.DeploymentBinding{}, err
		}
	}
	result, deployErr := adapter.Deploy(ctx, credential, gateways.DeployOptions{
		UpdateExisting: true,
		GroupIDs:       append([]int64(nil), target.DefaultGroupIDs...),
		Concurrency:    target.DefaultConcurrency,
		Priority:       target.DefaultPriority,
		RateMultiplier: target.RateMultiplier,
	})
	now := time.Now().UTC()
	binding := controlplane.DeploymentBinding{
		SubscriptionID: subscriptionID, TargetID: targetID, Mode: mode, Ownership: "managed",
		DesiredState: "active", ObservedState: "active", CredentialFingerprint: fingerprint(credential.Data), LastSyncedAt: &now,
	}
	if existing, existingErr := c.store.DeploymentBinding(ctx, subscriptionID, targetID); existingErr == nil {
		binding.RemoteAccountID = existing.RemoteAccountID
		binding.Ownership = defaultString(existing.Ownership, "managed")
	}
	if deployErr != nil {
		binding.ObservedState = "error"
		binding.LastError = cleanError(deployErr.Error())
		_, _ = c.store.UpsertDeploymentBinding(ctx, binding)
		if strings.TrimSpace(credential.LogicalIdentity) != "" {
			_ = c.store.CompleteCredentialAssignment(ctx, credential.LogicalIdentity, "failed", binding.LastError)
		}
		_ = c.store.CompleteSyncOperation(ctx, operation.ID, "failed", binding.LastError)
		return controlplane.DeploymentBinding{}, deployErr
	}
	binding.RemoteAccountID = result.Binding.ExternalID
	binding.LastError = ""
	persisted, err := c.store.UpsertDeploymentBinding(ctx, binding)
	if err != nil {
		if strings.TrimSpace(credential.LogicalIdentity) != "" {
			_ = c.store.CompleteCredentialAssignment(ctx, credential.LogicalIdentity, "failed", "persist binding failed")
		}
		_ = c.store.CompleteSyncOperation(ctx, operation.ID, "failed", "persist binding failed")
		return controlplane.DeploymentBinding{}, err
	}
	if strings.TrimSpace(credential.LogicalIdentity) != "" {
		if err := c.store.CompleteCredentialAssignment(ctx, credential.LogicalIdentity, "active", ""); err != nil {
			_ = c.store.CompleteSyncOperation(ctx, operation.ID, "failed", "persist assignment failed")
			return controlplane.DeploymentBinding{}, err
		}
	}
	if err := c.store.CompleteSyncOperation(ctx, operation.ID, "succeeded", ""); err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	return persisted, nil
}

func (c *Coordinator) Adopt(ctx context.Context, subscriptionID string, targetID int64, remoteAccountID string) (controlplane.DeploymentBinding, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.source.Get(subscriptionID); !ok {
		return controlplane.DeploymentBinding{}, errors.New("subscription was not found")
	}
	bindings, err := c.store.ListDeploymentBindings(ctx, subscriptionID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	for _, binding := range bindings {
		if binding.TargetID != targetID && binding.DesiredState == "active" && binding.ObservedState == "active" {
			return controlplane.DeploymentBinding{}, ErrActiveBindingExists
		}
	}
	target, err := c.store.GatewayTarget(ctx, targetID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	mode := "fallback"
	if target.Primary {
		mode = "primary"
	}
	now := time.Now().UTC()
	return c.store.UpsertDeploymentBinding(ctx, controlplane.DeploymentBinding{SubscriptionID: subscriptionID, TargetID: targetID, RemoteAccountID: strings.TrimSpace(remoteAccountID), Mode: mode, Ownership: "adopted", DesiredState: "active", ObservedState: "active", LastSyncedAt: &now})
}

func (c *Coordinator) Detach(ctx context.Context, subscriptionID string, targetID int64) (controlplane.DeploymentBinding, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.detach(ctx, subscriptionID, targetID)
}

func (c *Coordinator) detach(ctx context.Context, subscriptionID string, targetID int64) (controlplane.DeploymentBinding, error) {
	binding, err := c.store.DeploymentBinding(ctx, subscriptionID, targetID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	target, err := c.store.GatewayTarget(ctx, targetID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	credential, err := c.source.GatewayCredential(subscriptionID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	secret, err := c.store.GatewayTargetSecret(ctx, targetID)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	adapter, err := c.factory(target, secret)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	operation, err := c.store.CreateSyncOperation(ctx, controlplane.SyncOperation{SubscriptionID: subscriptionID, TargetID: targetID, Kind: "detach", Status: "running", Attempt: 1})
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	managed := binding.Ownership == "managed"
	if err := adapter.Detach(ctx, gateways.BindingRef{TargetID: strconvID(targetID), ExternalID: binding.RemoteAccountID, Managed: managed}, credential); err != nil {
		message := cleanError(err.Error())
		_ = c.store.CompleteSyncOperation(ctx, operation.ID, "failed", message)
		return controlplane.DeploymentBinding{}, err
	}
	now := time.Now().UTC()
	binding.DesiredState, binding.ObservedState, binding.LastError, binding.LastSyncedAt = "detached", "detached", "", &now
	persisted, err := c.store.UpsertDeploymentBinding(ctx, binding)
	if err != nil {
		return controlplane.DeploymentBinding{}, err
	}
	if strings.TrimSpace(credential.LogicalIdentity) != "" {
		if err := c.store.CompleteCredentialAssignment(ctx, credential.LogicalIdentity, "released", ""); err != nil {
			return controlplane.DeploymentBinding{}, err
		}
	}
	_ = c.store.CompleteSyncOperation(ctx, operation.ID, "succeeded", "")
	return persisted, nil
}

// Migrate detaches the source before activating the destination. If destination
// deployment fails, Orbit attempts to restore the source binding.
func (c *Coordinator) Migrate(ctx context.Context, subscriptionID string, fromTargetID, toTargetID int64) (controlplane.DeploymentBinding, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if fromTargetID == toTargetID {
		return c.deploy(ctx, subscriptionID, toTargetID)
	}
	if _, err := c.detach(ctx, subscriptionID, fromTargetID); err != nil {
		return controlplane.DeploymentBinding{}, fmt.Errorf("detach source gateway: %w", err)
	}
	binding, err := c.deploy(ctx, subscriptionID, toTargetID)
	if err == nil {
		return binding, nil
	}
	if _, rollbackErr := c.deploy(ctx, subscriptionID, fromTargetID); rollbackErr != nil {
		return controlplane.DeploymentBinding{}, fmt.Errorf("destination deployment failed and source rollback failed")
	}
	return controlplane.DeploymentBinding{}, fmt.Errorf("destination deployment failed; source binding restored: %w", err)
}

func (c *Coordinator) Bindings(ctx context.Context, subscriptionID string) ([]controlplane.DeploymentBinding, error) {
	return c.store.ListDeploymentBindings(ctx, subscriptionID)
}

// DetachAll removes every active runtime binding before its source archive is
// deleted. Managed bindings delete the resource created by Orbit; adopted
// bindings only clear the local relationship because adapters treat them as
// non-owned resources.
func (c *Coordinator) DetachAll(ctx context.Context, subscriptionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	bindings, err := c.store.ListDeploymentBindings(ctx, subscriptionID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		if binding.DesiredState != "active" && binding.ObservedState != "active" {
			continue
		}
		if strings.TrimSpace(binding.RemoteAccountID) == "" && binding.ObservedState != "active" {
			now := time.Now().UTC()
			binding.DesiredState, binding.ObservedState, binding.LastError, binding.LastSyncedAt = "detached", "detached", "", &now
			if _, err := c.store.UpsertDeploymentBinding(ctx, binding); err != nil {
				return err
			}
			continue
		}
		if _, err := c.detach(ctx, subscriptionID, binding.TargetID); err != nil {
			return fmt.Errorf("detach target %d: %w", binding.TargetID, err)
		}
	}
	return nil
}

func (c *Coordinator) Operations(ctx context.Context, limit int) ([]controlplane.SyncOperation, error) {
	return c.store.ListSyncOperations(ctx, limit)
}

func (c *Coordinator) Inspect(ctx context.Context, subscriptionID string) (model.Connectivity, error) {
	bindings, err := c.store.ListDeploymentBindings(ctx, subscriptionID)
	if err != nil {
		return model.Connectivity{}, err
	}
	var selected *controlplane.DeploymentBinding
	for i := range bindings {
		binding := &bindings[i]
		if binding.ObservedState != "active" || binding.DesiredState != "active" {
			continue
		}
		if selected == nil || (binding.Mode == "primary" && selected.Mode != "primary") {
			selected = binding
		}
	}
	if selected == nil {
		credential, credentialErr := c.source.GatewayCredential(subscriptionID)
		if credentialErr != nil {
			return model.Connectivity{}, credentialErr
		}
		if gateways.IsSub2APIDataPackage(credential.Data) {
			return model.Connectivity{Status: "pending", ReasonCode: "sub2api_not_deployed", Error: "尚未部署到 Sub2API 号池", CheckedAt: time.Now()}, nil
		}
		return model.Connectivity{}, sql.ErrNoRows
	}
	target, err := c.store.GatewayTarget(ctx, selected.TargetID)
	if err != nil {
		return model.Connectivity{}, err
	}
	credential, err := c.source.GatewayCredential(subscriptionID)
	if err != nil {
		return model.Connectivity{}, err
	}
	secret, err := c.store.GatewayTargetSecret(ctx, target.ID)
	if err != nil {
		return model.Connectivity{}, err
	}
	adapter, err := c.factory(target, secret)
	if err != nil {
		return model.Connectivity{}, err
	}
	result, err := adapter.Inspect(ctx, gateways.BindingRef{TargetID: strconvID(target.ID), ExternalID: selected.RemoteAccountID, Managed: selected.Ownership == "managed"}, credential)
	return result.Connectivity, err
}

func fingerprint(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func cleanError(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if len(message) > 240 {
		message = message[:240] + "…"
	}
	return message
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func strconvID(value int64) string { return fmt.Sprintf("%d", value) }
