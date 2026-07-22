package accounthealth

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/model"
)

type fakeInspector struct {
	bindings []controlplane.DeploymentBinding
	block    <-chan struct{}
	mu       sync.Mutex
	calls    []string
	current  int
	maximum  int
}

func (f *fakeInspector) Bindings(context.Context, string) ([]controlplane.DeploymentBinding, error) {
	return append([]controlplane.DeploymentBinding(nil), f.bindings...), nil
}

func (f *fakeInspector) Inspect(ctx context.Context, id string) (model.Connectivity, error) {
	f.mu.Lock()
	f.calls = append(f.calls, id)
	f.current++
	if f.current > f.maximum {
		f.maximum = f.current
	}
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.current--
		f.mu.Unlock()
	}()
	if f.block != nil {
		select {
		case <-ctx.Done():
			return model.Connectivity{}, ctx.Err()
		case <-f.block:
		}
	}
	return model.Connectivity{Status: "ok", CheckedAt: time.Now()}, nil
}

type fakeConnectivityStore struct {
	mu     sync.Mutex
	checks map[string]model.Connectivity
}

func (f *fakeConnectivityStore) SaveConnectivity(id string, check model.Connectivity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.checks == nil {
		f.checks = make(map[string]model.Connectivity)
	}
	f.checks[id] = check
	return nil
}

func newSettings(t *testing.T, minutes int) *config.Store {
	t.Helper()
	store, err := config.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	settings := store.Get()
	settings.AccountPollMinutes = minutes
	if err := store.Update(settings); err != nil {
		t.Fatal(err)
	}
	return store
}

func waitFinished(t *testing.T, scheduler *Scheduler) Status {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		status := scheduler.Status()
		if status.RunsCompleted > 0 {
			return status
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("poll did not finish")
	return Status{}
}

func TestSchedulerImmediatelyPollsOnlyUniqueActiveAssignments(t *testing.T) {
	inspector := &fakeInspector{bindings: []controlplane.DeploymentBinding{
		{SubscriptionID: "active", DesiredState: "active", ObservedState: "active"},
		{SubscriptionID: "active", DesiredState: "active", ObservedState: "active"},
		{SubscriptionID: "desired-only", DesiredState: "active", ObservedState: "error"},
		{SubscriptionID: "detached", DesiredState: "detached", ObservedState: "detached"},
	}}
	saved := &fakeConnectivityStore{}
	scheduler := NewScheduler(newSettings(t, 5), inspector, saved)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler.Start(ctx)
	status := waitFinished(t, scheduler)
	if status.TotalAccounts != 1 || status.Completed != 1 || status.Succeeded != 1 || status.Failed != 0 {
		t.Fatalf("unexpected status: %+v", status)
	}
	inspector.mu.Lock()
	defer inspector.mu.Unlock()
	if len(inspector.calls) != 1 || inspector.calls[0] != "active" {
		t.Fatalf("inspect calls = %v", inspector.calls)
	}
	saved.mu.Lock()
	defer saved.mu.Unlock()
	if saved.checks["active"].Status != "ok" {
		t.Fatalf("connectivity was not persisted: %+v", saved.checks)
	}
}

func TestSchedulerDisabledSkipsImmediateRunButAllowsManualPoll(t *testing.T) {
	inspector := &fakeInspector{bindings: []controlplane.DeploymentBinding{{SubscriptionID: "active", DesiredState: "active", ObservedState: "active"}}}
	scheduler := NewScheduler(newSettings(t, 0), inspector, &fakeConnectivityStore{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	if status := scheduler.Status(); status.Enabled || status.RunsStarted != 0 || status.NextRunAt != nil {
		t.Fatalf("disabled status = %+v", status)
	}
	if err := scheduler.PollNow(); err != nil {
		t.Fatal(err)
	}
	status := waitFinished(t, scheduler)
	if status.RunsCompleted != 1 {
		t.Fatalf("manual poll status = %+v", status)
	}
}

func TestSchedulerBoundsConcurrencyAndRejectsOverlap(t *testing.T) {
	release := make(chan struct{})
	bindings := make([]controlplane.DeploymentBinding, 10)
	for i := range bindings {
		bindings[i] = controlplane.DeploymentBinding{SubscriptionID: string(rune('a' + i)), DesiredState: "active", ObservedState: "active"}
	}
	inspector := &fakeInspector{bindings: bindings, block: release}
	scheduler := NewScheduler(newSettings(t, 0), inspector, &fakeConnectivityStore{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	scheduler.Start(ctx)
	if err := scheduler.PollNow(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		inspector.mu.Lock()
		current := inspector.current
		inspector.mu.Unlock()
		if current == maxConcurrency {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := scheduler.PollNow(); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("overlapping poll error = %v", err)
	}
	close(release)
	status := waitFinished(t, scheduler)
	inspector.mu.Lock()
	maximum := inspector.maximum
	inspector.mu.Unlock()
	if maximum != maxConcurrency || status.Completed != len(bindings) {
		t.Fatalf("maximum concurrency=%d status=%+v", maximum, status)
	}
}
