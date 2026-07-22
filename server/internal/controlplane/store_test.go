package controlplane

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersistsTargetsBindingsOperationsAndUsage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := NewStore(filepath.Join(t.TempDir(), "control-plane.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	target, err := store.UpsertGatewayTarget(ctx, GatewayTarget{
		Kind: "sub2api", Name: "Local Sub2API", BaseURL: "http://127.0.0.1:8081",
		AdminKey: "admin-secret", Enabled: true, Primary: true, DefaultGroupIDs: []int64{3, 5},
		DefaultConcurrency: 2, DefaultPriority: 8, RateMultiplier: 1.25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.ID == 0 || target.AdminKey != "" || !target.AdminKeyConfigured || len(target.DefaultGroupIDs) != 2 || target.DefaultConcurrency != 2 || target.RateMultiplier != 1.25 {
		t.Fatalf("target must be persisted and redacted: %+v", target)
	}
	fallback, err := store.UpsertGatewayTarget(ctx, GatewayTarget{Kind: "cpa", Name: "Fallback", BaseURL: "http://127.0.0.1:8317/v1", Enabled: true, Primary: true})
	if err != nil {
		t.Fatal(err)
	}
	if !fallback.Primary {
		t.Fatal("new primary target was not persisted")
	}

	targets, err := store.ListGatewayTargets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 || targets[0].ID != fallback.ID || !targets[0].Primary || targets[1].Primary || targets[1].AdminKey != "" || !targets[1].AdminKeyConfigured {
		t.Fatalf("listed targets must not expose secrets: %+v", targets)
	}
	secret, err := store.GatewayTargetSecret(ctx, target.ID)
	if err != nil || secret != "admin-secret" {
		t.Fatalf("internal secret lookup failed: secret=%q err=%v", secret, err)
	}

	binding, err := store.UpsertDeploymentBinding(ctx, DeploymentBinding{
		SubscriptionID: "subscription-1", TargetID: target.ID, RemoteAccountID: "42",
		Mode: "primary", Ownership: "managed", DesiredState: "active", ObservedState: "active",
		CredentialFingerprint: "sha256:one",
	})
	if err != nil {
		t.Fatal(err)
	}
	if binding.ID == 0 {
		t.Fatal("binding ID was not assigned")
	}
	bindings, err := store.ListDeploymentBindings(ctx, "subscription-1")
	if err != nil || len(bindings) != 1 || bindings[0].RemoteAccountID != "42" {
		t.Fatalf("binding round trip failed: bindings=%+v err=%v", bindings, err)
	}

	operation, err := store.CreateSyncOperation(ctx, SyncOperation{
		SubscriptionID: "subscription-1", TargetID: target.ID, Kind: "deploy", Status: "pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteSyncOperation(ctx, operation.ID, "succeeded", ""); err != nil {
		t.Fatal(err)
	}
	operations, err := store.ListSyncOperations(ctx, 10)
	if err != nil || len(operations) != 1 || operations[0].Status != "succeeded" || operations[0].CompletedAt == nil {
		t.Fatalf("operation completion failed: operations=%+v err=%v", operations, err)
	}

	bucketAt := time.Date(2026, 7, 22, 9, 15, 0, 0, time.UTC)
	bucket := UsageBucket{
		TargetID: target.ID, BucketAt: bucketAt, BucketMinutes: 15, AccountID: "42", Model: "gpt-5",
		Requests: 2, InputTokens: 100, OutputTokens: 50, CacheReadTokens: 10, Cost: 0.25,
	}
	if err := store.UpsertUsageBucket(ctx, bucket); err != nil {
		t.Fatal(err)
	}
	bucket.Requests = 3
	bucket.OutputTokens = 75
	if err := store.UpsertUsageBucket(ctx, bucket); err != nil {
		t.Fatal(err)
	}
	trend, err := store.ListUsageBuckets(ctx, target.ID, bucketAt.Add(-time.Minute), bucketAt.Add(time.Minute))
	if err != nil || len(trend) != 1 || trend[0].Requests != 3 || trend[0].OutputTokens != 75 {
		t.Fatalf("usage upsert failed: trend=%+v err=%v", trend, err)
	}
}
