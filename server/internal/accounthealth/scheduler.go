package accounthealth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/controlplane"
	"cpa-monitor/server/internal/model"
)

const (
	maxConcurrency    = 4
	perAccountTimeout = 30 * time.Second
)

var (
	ErrAlreadyRunning = errors.New("account polling is already running")
	ErrNotStarted     = errors.New("account polling has not started")
)

type Inspector interface {
	Bindings(context.Context, string) ([]controlplane.DeploymentBinding, error)
	Inspect(context.Context, string) (model.Connectivity, error)
}

type ConnectivityStore interface {
	SaveConnectivity(string, model.Connectivity) error
}

type Status struct {
	Enabled         bool       `json:"enabled"`
	Running         bool       `json:"running"`
	IntervalMinutes int        `json:"intervalMinutes"`
	NextRunAt       *time.Time `json:"nextRunAt,omitempty"`
	LastStartedAt   *time.Time `json:"lastStartedAt,omitempty"`
	LastFinishedAt  *time.Time `json:"lastFinishedAt,omitempty"`
	TotalAccounts   int        `json:"totalAccounts"`
	Completed       int        `json:"completed"`
	Succeeded       int        `json:"succeeded"`
	Failed          int        `json:"failed"`
	RunsStarted     uint64     `json:"runsStarted"`
	RunsCompleted   uint64     `json:"runsCompleted"`
	LastError       string     `json:"lastError,omitempty"`
}

type Scheduler struct {
	settings  *config.Store
	inspector Inspector
	store     ConnectivityStore

	mu      sync.Mutex
	status  Status
	started bool
	ctx     context.Context
	reset   chan struct{}
}

func NewScheduler(settings *config.Store, inspector Inspector, store ConnectivityStore) *Scheduler {
	minutes := settings.Get().AccountPollMinutes
	return &Scheduler{
		settings: settings, inspector: inspector, store: store,
		status: Status{Enabled: minutes != 0, IntervalMinutes: minutes},
		reset:  make(chan struct{}, 1),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.ctx = ctx
	s.mu.Unlock()
	go s.schedule(ctx)
}

func (s *Scheduler) schedule(ctx context.Context) {
	minutes := s.configureNextRun(true)
	var timer *time.Timer
	var timerC <-chan time.Time
	if minutes > 0 {
		timer = time.NewTimer(time.Duration(minutes) * time.Minute)
		timerC = timer.C
	}
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.status.NextRunAt = nil
			s.mu.Unlock()
			return
		case <-s.reset:
			if timer != nil {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
			}
			minutes = s.configureNextRun(false)
			if minutes == 0 {
				timerC = nil
				continue
			}
			if timer == nil {
				timer = time.NewTimer(time.Duration(minutes) * time.Minute)
			} else {
				timer.Reset(time.Duration(minutes) * time.Minute)
			}
			timerC = timer.C
		case <-timerC:
			_ = s.startRun()
			minutes = s.configureNextRun(false)
			if minutes == 0 {
				timerC = nil
				continue
			}
			timer.Reset(time.Duration(minutes) * time.Minute)
			timerC = timer.C
		}
	}
}

func (s *Scheduler) configureNextRun(immediate bool) int {
	minutes := s.settings.Get().AccountPollMinutes
	s.mu.Lock()
	s.status.Enabled = minutes != 0
	s.status.IntervalMinutes = minutes
	if minutes == 0 {
		s.status.NextRunAt = nil
	} else {
		next := time.Now().Add(time.Duration(minutes) * time.Minute)
		s.status.NextRunAt = &next
	}
	s.mu.Unlock()
	if immediate && minutes != 0 {
		_ = s.startRun()
	}
	return minutes
}

func (s *Scheduler) ResetSchedule() {
	minutes := s.settings.Get().AccountPollMinutes
	s.mu.Lock()
	s.status.Enabled = minutes != 0
	s.status.IntervalMinutes = minutes
	if minutes == 0 {
		s.status.NextRunAt = nil
	}
	s.mu.Unlock()
	select {
	case s.reset <- struct{}{}:
	default:
	}
}

func (s *Scheduler) PollNow() error {
	return s.startRun()
}

func (s *Scheduler) startRun() error {
	s.mu.Lock()
	if !s.started || s.ctx == nil {
		s.mu.Unlock()
		return ErrNotStarted
	}
	if s.status.Running {
		s.mu.Unlock()
		return ErrAlreadyRunning
	}
	now := time.Now()
	s.status.Running = true
	s.status.LastStartedAt = &now
	s.status.LastFinishedAt = nil
	s.status.TotalAccounts = 0
	s.status.Completed = 0
	s.status.Succeeded = 0
	s.status.Failed = 0
	s.status.LastError = ""
	s.status.RunsStarted++
	ctx := s.ctx
	s.mu.Unlock()
	go s.run(ctx)
	return nil
}

func (s *Scheduler) run(ctx context.Context) {
	bindings, err := s.inspector.Bindings(ctx, "")
	if err != nil {
		s.finishRun(err)
		return
	}
	ids := activeSubscriptionIDs(bindings)
	s.mu.Lock()
	s.status.TotalAccounts = len(ids)
	s.mu.Unlock()

	jobs := make(chan string)
	var workers sync.WaitGroup
	workerCount := maxConcurrency
	if len(ids) < workerCount {
		workerCount = len(ids)
	}
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				s.pollAccount(ctx, id)
			}
		}()
	}

sendJobs:
	for _, id := range ids {
		select {
		case <-ctx.Done():
			break sendJobs
		case jobs <- id:
		}
	}
	close(jobs)
	workers.Wait()
	if ctx.Err() != nil {
		s.finishRun(ctx.Err())
		return
	}
	s.finishRun(nil)
}

func (s *Scheduler) pollAccount(ctx context.Context, id string) {
	accountCtx, cancel := context.WithTimeout(ctx, perAccountTimeout)
	check, err := s.inspector.Inspect(accountCtx, id)
	cancel()
	if err == nil {
		err = s.store.SaveConnectivity(id, check)
	}
	s.mu.Lock()
	s.status.Completed++
	if err != nil {
		s.status.Failed++
	} else {
		s.status.Succeeded++
	}
	s.mu.Unlock()
}

func (s *Scheduler) finishRun(err error) {
	now := time.Now()
	s.mu.Lock()
	s.status.Running = false
	s.status.LastFinishedAt = &now
	s.status.RunsCompleted++
	if err != nil {
		s.status.LastError = cleanError(err.Error())
	}
	s.mu.Unlock()
}

func (s *Scheduler) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func activeSubscriptionIDs(bindings []controlplane.DeploymentBinding) []string {
	seen := make(map[string]struct{})
	ids := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		id := strings.TrimSpace(binding.SubscriptionID)
		if id == "" || binding.DesiredState != "active" || binding.ObservedState != "active" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func cleanError(message string) string {
	message = strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if len(message) > 240 {
		return message[:240] + "…"
	}
	return message
}
