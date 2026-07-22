package observability

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/gateways/sub2api"
)

type fakeSource struct {
	snapshot sub2api.Snapshot
	usage    sub2api.UsagePage
	err      error
}

func (f *fakeSource) DashboardSnapshot(context.Context) (sub2api.Snapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshot, nil
}
func (f *fakeSource) UsageRange(context.Context, int, int, time.Time, time.Time) (sub2api.UsagePage, error) {
	if f.err != nil {
		return sub2api.UsagePage{}, f.err
	}
	return f.usage, nil
}

func TestCollectPersistsFifteenMinuteUsageAndPreservesLastSnapshot(t *testing.T) {
	store, err := controlplane.NewStore(filepath.Join(t.TempDir(), "control.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ctx := context.Background()
	target, err := store.UpsertGatewayTarget(ctx, controlplane.GatewayTarget{Kind: "sub2api", Name: "primary", BaseURL: "http://127.0.0.1:8080", Enabled: true, Primary: true})
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Minute)
	row, _ := json.Marshal(map[string]any{"account_id": 42, "model": "gpt-5", "group": map[string]any{"name": "plus"}, "input_tokens": 100, "output_tokens": 30, "cache_read_tokens": 10, "total_cost": 0.2, "actual_cost": 0.1, "duration_ms": 1200, "first_token_ms": 300, "created_at": createdAt.Format(time.RFC3339)})
	source := &fakeSource{snapshot: sub2api.Snapshot{"total_requests": float64(1), "total_tokens": float64(140)}, usage: sub2api.UsagePage{Items: []json.RawMessage{row}, Page: 1, Pages: 1}}
	collector := NewCollector(store, func(controlplane.GatewayTarget, string) (Source, error) { return source, nil })
	if err := collector.Collect(ctx); err != nil {
		t.Fatal(err)
	}
	bucketAt := createdAt.Truncate(15 * time.Minute)
	buckets, err := store.ListUsageBuckets(ctx, target.ID, bucketAt.Add(-time.Minute), bucketAt.Add(16*time.Minute))
	if err != nil || len(buckets) != 1 {
		t.Fatalf("usage buckets=%+v err=%v", buckets, err)
	}
	if buckets[0].Requests != 1 || buckets[0].InputTokens != 100 || buckets[0].OutputTokens != 30 || buckets[0].CacheReadTokens != 10 || buckets[0].AverageDurationMS != 1200 {
		t.Fatalf("unexpected normalized bucket: %+v", buckets[0])
	}

	source.err = errors.New("offline")
	if err := collector.Collect(ctx); err == nil {
		t.Fatal("expected partial collection error")
	}
	state, ok := collector.Snapshot(target.ID)
	if !ok || !state.Stale || state.Data["total_requests"] != float64(1) || state.LastSuccessAt.IsZero() {
		t.Fatalf("last valid snapshot was not preserved: %+v", state)
	}
}
