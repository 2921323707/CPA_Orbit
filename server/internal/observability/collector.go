package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/gateways"
	"cpa-monitor/server/internal/gateways/sub2api"
)

const (
	bucketSize    = 15 * time.Minute
	retentionDays = 90
)

type Source interface {
	DashboardSnapshot(context.Context) (sub2api.Snapshot, error)
	UsageRange(context.Context, int, int, time.Time, time.Time) (sub2api.UsagePage, error)
}

type SourceFactory func(controlplane.GatewayTarget, string) (Source, error)

type SnapshotState struct {
	TargetID      int64            `json:"targetId"`
	Data          sub2api.Snapshot `json:"data"`
	Stale         bool             `json:"stale"`
	LastError     string           `json:"lastError,omitempty"`
	LastAttemptAt time.Time        `json:"lastAttemptAt"`
	LastSuccessAt time.Time        `json:"lastSuccessAt,omitempty"`
}

type Collector struct {
	store   *controlplane.Store
	factory SourceFactory
	mu      sync.RWMutex
	states  map[int64]SnapshotState
}

func NewCollector(store *controlplane.Store, factory SourceFactory) *Collector {
	return &Collector{store: store, factory: factory, states: make(map[int64]SnapshotState)}
}

func (c *Collector) Start(ctx context.Context) {
	go func() {
		_ = c.Collect(ctx)
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = c.Collect(ctx)
			}
		}
	}()
}

func (c *Collector) Collect(ctx context.Context) error {
	targets, err := c.store.ListGatewayTargets(ctx)
	if err != nil {
		return err
	}
	var collectionErrors []error
	for _, target := range targets {
		if !target.Enabled || target.Kind != string(gateways.KindSub2API) {
			continue
		}
		secret, err := c.store.GatewayTargetSecret(ctx, target.ID)
		if err != nil {
			collectionErrors = append(collectionErrors, err)
			continue
		}
		source, err := c.factory(target, secret)
		if err != nil {
			c.markStale(target.ID)
			collectionErrors = append(collectionErrors, err)
			continue
		}
		now := time.Now().UTC()
		snapshot, snapshotErr := source.DashboardSnapshot(ctx)
		if snapshotErr != nil {
			c.markStale(target.ID)
			collectionErrors = append(collectionErrors, fmt.Errorf("target %d snapshot: %w", target.ID, snapshotErr))
		} else {
			c.mu.Lock()
			c.states[target.ID] = SnapshotState{TargetID: target.ID, Data: cloneSnapshot(snapshot), LastAttemptAt: now, LastSuccessAt: now}
			c.mu.Unlock()
		}
		to := now.Truncate(bucketSize)
		from := to.Add(-2 * time.Hour)
		if usageErr := c.collectUsage(ctx, source, target.ID, from, to); usageErr != nil {
			collectionErrors = append(collectionErrors, fmt.Errorf("target %d usage: %w", target.ID, usageErr))
		}
	}
	_, retentionErr := c.store.DeleteUsageBefore(ctx, time.Now().UTC().Add(-retentionDays*24*time.Hour))
	if retentionErr != nil {
		collectionErrors = append(collectionErrors, retentionErr)
	}
	return errors.Join(collectionErrors...)
}

func (c *Collector) collectUsage(ctx context.Context, source Source, targetID int64, from, to time.Time) error {
	aggregates := make(map[string]*usageAggregate)
	for page := 1; page <= 20; page++ {
		result, err := source.UsageRange(ctx, page, 500, from, to)
		if err != nil {
			return err
		}
		for _, item := range result.Items {
			row, err := decodeUsage(item)
			if err != nil || row.CreatedAt.Before(from) || !row.CreatedAt.Before(to) {
				continue
			}
			key := fmt.Sprintf("%d|%s|%s|%s", row.CreatedAt.Truncate(bucketSize).Unix(), row.AccountID, row.GroupName, row.Model)
			aggregate := aggregates[key]
			if aggregate == nil {
				aggregate = &usageAggregate{Bucket: controlplane.UsageBucket{TargetID: targetID, BucketAt: row.CreatedAt.Truncate(bucketSize), BucketMinutes: 15, AccountID: row.AccountID, GroupName: row.GroupName, Model: row.Model}}
				aggregates[key] = aggregate
			}
			aggregate.add(row)
		}
		if result.Pages <= page || len(result.Items) < 500 {
			break
		}
	}
	for _, aggregate := range aggregates {
		aggregate.finish()
		if err := c.store.UpsertUsageBucket(ctx, aggregate.Bucket); err != nil {
			return err
		}
	}
	return nil
}

func (c *Collector) Snapshot(targetID int64) (SnapshotState, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state, ok := c.states[targetID]
	state.Data = cloneSnapshot(state.Data)
	return state, ok
}

func (c *Collector) Snapshots() []SnapshotState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]SnapshotState, 0, len(c.states))
	for _, state := range c.states {
		state.Data = cloneSnapshot(state.Data)
		result = append(result, state)
	}
	return result
}

func (c *Collector) markStale(targetID int64) {
	now := time.Now().UTC()
	c.mu.Lock()
	state := c.states[targetID]
	state.TargetID, state.Stale, state.LastAttemptAt, state.LastError = targetID, true, now, "Sub2API collection failed"
	c.states[targetID] = state
	c.mu.Unlock()
}

type usageRow struct {
	CreatedAt           time.Time
	AccountID           string
	GroupName           string
	Model               string
	InputTokens         int64
	OutputTokens        int64
	CacheCreationTokens int64
	CacheReadTokens     int64
	Cost                float64
	ActualCost          float64
	DurationMS          float64
	FirstTokenMS        float64
	Failed              bool
}

type usageAggregate struct {
	Bucket        controlplane.UsageBucket
	durationTotal float64
	firstTotal    float64
	durationCount int64
	firstCount    int64
}

func (a *usageAggregate) add(row usageRow) {
	a.Bucket.Requests++
	if row.Failed {
		a.Bucket.Failures++
	} else {
		a.Bucket.Successes++
	}
	a.Bucket.InputTokens += row.InputTokens
	a.Bucket.OutputTokens += row.OutputTokens
	a.Bucket.CacheCreationTokens += row.CacheCreationTokens
	a.Bucket.CacheReadTokens += row.CacheReadTokens
	a.Bucket.Cost += row.Cost
	a.Bucket.ActualCost += row.ActualCost
	if row.DurationMS > 0 {
		a.durationTotal += row.DurationMS
		a.durationCount++
	}
	if row.FirstTokenMS > 0 {
		a.firstTotal += row.FirstTokenMS
		a.firstCount++
	}
}

func (a *usageAggregate) finish() {
	if a.durationCount > 0 {
		a.Bucket.AverageDurationMS = a.durationTotal / float64(a.durationCount)
	}
	if a.firstCount > 0 {
		a.Bucket.FirstTokenMS = a.firstTotal / float64(a.firstCount)
	}
}

func decodeUsage(data json.RawMessage) (usageRow, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return usageRow{}, err
	}
	createdAt, err := parseCreatedAt(raw["created_at"])
	if err != nil {
		return usageRow{}, err
	}
	row := usageRow{
		CreatedAt: createdAt, AccountID: stringValue(raw["account_id"]), Model: stringValue(raw["model"]),
		InputTokens: intValue(raw["input_tokens"]), OutputTokens: intValue(raw["output_tokens"]),
		CacheCreationTokens: intValue(raw["cache_creation_tokens"]), CacheReadTokens: intValue(raw["cache_read_tokens"]),
		Cost: floatValue(raw["total_cost"]), ActualCost: floatValue(raw["actual_cost"]),
		DurationMS: floatValue(raw["duration_ms"]), FirstTokenMS: floatValue(raw["first_token_ms"]),
	}
	row.GroupName = stringValue(raw["group_name"])
	if group, ok := raw["group"].(map[string]any); ok {
		row.GroupName = firstNonEmpty(stringValue(group["name"]), row.GroupName)
	}
	status := intValue(raw["status_code"])
	row.Failed = status >= 400
	return row, nil
}

func parseCreatedAt(value any) (time.Time, error) {
	if text := stringValue(value); text != "" {
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				return parsed.UTC(), nil
			}
		}
	}
	if unix := intValue(value); unix > 0 {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Time{}, errors.New("usage row has no valid created_at")
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	default:
		return ""
	}
}

func intValue(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		result, _ := typed.Int64()
		return result
	case float64:
		return int64(typed)
	case string:
		result, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return result
	default:
		return 0
	}
}

func floatValue(value any) float64 {
	switch typed := value.(type) {
	case json.Number:
		result, _ := typed.Float64()
		return result
	case float64:
		return typed
	case string:
		result, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return result
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func cloneSnapshot(source sub2api.Snapshot) sub2api.Snapshot {
	if source == nil {
		return nil
	}
	result := make(sub2api.Snapshot, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
