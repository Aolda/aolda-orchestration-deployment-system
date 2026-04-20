package application

import (
	"strings"
	"sync"
	"time"

	"github.com/aolda/aods-backend/internal/project"
)

type RepositoryPollResult string

const (
	RepositoryPollResultPending RepositoryPollResult = "Pending"
	RepositoryPollResultSuccess RepositoryPollResult = "Success"
	RepositoryPollResultError   RepositoryPollResult = "Error"
)

type RepositoryPollStatus struct {
	Enabled         bool                 `json:"enabled"`
	IntervalSeconds int                  `json:"intervalSeconds"`
	LastCheckedAt   *time.Time           `json:"lastCheckedAt,omitempty"`
	LastSucceededAt *time.Time           `json:"lastSucceededAt,omitempty"`
	NextScheduledAt *time.Time           `json:"nextScheduledAt,omitempty"`
	LastResult      RepositoryPollResult `json:"lastResult,omitempty"`
	LastError       string               `json:"lastError,omitempty"`
	Source          string               `json:"source,omitempty"`
}

type RepositoryPollTracker struct {
	mu          sync.RWMutex
	interval    time.Duration
	startedAt   time.Time
	nextCycleAt time.Time
	items       map[string]RepositoryPollStatus
}

func NewRepositoryPollTracker(interval time.Duration) *RepositoryPollTracker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	now := timeNowUTC()
	return &RepositoryPollTracker{
		interval:    interval,
		startedAt:   now,
		nextCycleAt: now.Add(interval),
		items:       make(map[string]RepositoryPollStatus),
	}
}

func (t *RepositoryPollTracker) BeginCycle(now time.Time) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.nextCycleAt = now.UTC().Add(t.interval)
}

func (t *RepositoryPollTracker) MarkAttempt(app Record, repo project.Repository, checkedAt time.Time) {
	if t == nil || !appHasRepositorySource(app) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	interval := repositoryPollIntervalForApp(app, t.interval)
	current := t.items[app.ID]
	current.Enabled = true
	current.IntervalSeconds = int(interval.Seconds())
	current.LastCheckedAt = timePointer(checkedAt.UTC())
	current.NextScheduledAt = timePointer(checkedAt.UTC().Add(interval))
	current.LastResult = RepositoryPollResultPending
	current.LastError = ""
	current.Source = repositoryPollSource(repo, app)
	t.items[app.ID] = current
}

func (t *RepositoryPollTracker) MarkSuccess(app Record, repo project.Repository, checkedAt time.Time) {
	if t == nil || !appHasRepositorySource(app) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	interval := repositoryPollIntervalForApp(app, t.interval)
	current := t.items[app.ID]
	current.Enabled = true
	current.IntervalSeconds = int(interval.Seconds())
	current.LastCheckedAt = timePointer(checkedAt.UTC())
	current.LastSucceededAt = timePointer(checkedAt.UTC())
	current.NextScheduledAt = timePointer(checkedAt.UTC().Add(interval))
	current.LastResult = RepositoryPollResultSuccess
	current.LastError = ""
	current.Source = repositoryPollSource(repo, app)
	t.items[app.ID] = current
}

func (t *RepositoryPollTracker) MarkFailure(app Record, repo project.Repository, checkedAt time.Time, err error) {
	if t == nil || !appHasRepositorySource(app) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	interval := repositoryPollIntervalForApp(app, t.interval)
	current := t.items[app.ID]
	current.Enabled = true
	current.IntervalSeconds = int(interval.Seconds())
	current.LastCheckedAt = timePointer(checkedAt.UTC())
	current.NextScheduledAt = timePointer(checkedAt.UTC().Add(interval))
	current.LastResult = RepositoryPollResultError
	if err != nil {
		current.LastError = err.Error()
	}
	current.Source = repositoryPollSource(repo, app)
	t.items[app.ID] = current
}

func (t *RepositoryPollTracker) Reschedule(app Record, scheduledFrom time.Time) {
	if t == nil || !appHasRepositorySource(app) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	interval := repositoryPollIntervalForApp(app, t.interval)
	current := t.items[app.ID]
	current.Enabled = true
	current.IntervalSeconds = int(interval.Seconds())
	current.NextScheduledAt = timePointer(scheduledFrom.UTC().Add(interval))
	if current.Source == "" {
		current.Source = repositoryPollSource(project.Repository{}, app)
	}
	t.items[app.ID] = current
}

func (t *RepositoryPollTracker) Due(app Record, now time.Time) bool {
	if t == nil || !appHasRepositorySource(app) {
		return false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	interval := repositoryPollIntervalForApp(app, t.interval)
	current, ok := t.items[app.ID]
	if !ok || current.LastCheckedAt == nil || current.LastCheckedAt.IsZero() {
		return !t.startedAt.Add(interval).After(now.UTC())
	}

	return !current.LastCheckedAt.UTC().Add(interval).After(now.UTC())
}

func (t *RepositoryPollTracker) Snapshot(app Record) *RepositoryPollStatus {
	if t == nil || !appHasRepositorySource(app) {
		return nil
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if current, ok := t.items[app.ID]; ok {
		cloned := current
		if cloned.IntervalSeconds == 0 {
			cloned.IntervalSeconds = int(repositoryPollIntervalForApp(app, t.interval).Seconds())
		}
		if cloned.Source == "" {
			cloned.Source = repositoryPollSource(project.Repository{}, app)
		}
		return &cloned
	}

	interval := repositoryPollIntervalForApp(app, t.interval)
	return &RepositoryPollStatus{
		Enabled:         true,
		IntervalSeconds: int(interval.Seconds()),
		NextScheduledAt: timePointer(t.startedAt.Add(interval)),
		LastResult:      RepositoryPollResultPending,
		Source:          repositoryPollSource(project.Repository{}, app),
	}
}

func (t *RepositoryPollTracker) DefaultInterval() time.Duration {
	if t == nil || t.interval <= 0 {
		return 5 * time.Minute
	}
	return t.interval
}

func appHasRepositorySource(app Record) bool {
	return strings.TrimSpace(app.RepositoryURL) != "" || strings.TrimSpace(app.RepositoryID) != ""
}

func repositoryPollSource(repo project.Repository, app Record) string {
	switch {
	case strings.TrimSpace(repo.ConfigFile) != "":
		return strings.TrimSpace(repo.ConfigFile)
	case strings.TrimSpace(app.ConfigPath) != "":
		return strings.TrimSpace(app.ConfigPath)
	default:
		return DefaultRepositoryConfigPath
	}
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func normalizeRepositoryPollIntervalSeconds(value int) int {
	for _, allowed := range AllowedRepositoryPollIntervalsSeconds {
		if value == allowed {
			return value
		}
	}
	return 0
}

func repositoryPollIntervalForApp(app Record, fallback time.Duration) time.Duration {
	if normalized := normalizeRepositoryPollIntervalSeconds(app.RepositoryPollIntervalSeconds); normalized > 0 {
		return time.Duration(normalized) * time.Second
	}
	if fallback <= 0 {
		return 5 * time.Minute
	}
	return fallback
}
