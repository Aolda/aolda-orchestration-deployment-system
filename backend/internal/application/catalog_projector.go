package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/aolda/aods-backend/internal/project"
)

type ApplicationCatalogProjector struct {
	Store    ApplicationCatalogRefreshStore
	Projects project.CatalogSource
	Interval time.Duration
	Timeout  time.Duration
}

func (p *ApplicationCatalogProjector) Start(ctx context.Context) {
	if p == nil || p.Store == nil || p.Projects == nil {
		return
	}
	interval := p.Interval
	if interval <= 0 {
		return
	}

	slog.Info("starting application catalog projector", "interval", interval, "timeout", p.effectiveTimeout())
	p.refreshAll(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.refreshAll(ctx)
		}
	}
}

func (p *ApplicationCatalogProjector) refreshAll(ctx context.Context) {
	startedAt := time.Now()
	runCtx, cancel, timeout := backgroundWorkerContext(ctx, p.Timeout, defaultApplicationCatalogProjectorTimeout)
	defer cancel()

	projects, err := p.Projects.ListProjects(runCtx)
	duration := time.Since(startedAt)
	if err != nil {
		if reason := backgroundWorkerSkipReason(err); reason != "" {
			slog.Info("application catalog projection skipped", "reason", reason, "timeout", timeout, "duration", duration)
			return
		}
		slog.Warn("application catalog projection project list failed", "duration", duration, "error", err)
		return
	}

	refreshed := 0
	skipped := 0
	failed := 0
	for _, item := range projects {
		if err := runCtx.Err(); err != nil {
			slog.Info("application catalog projection skipped", "reason", backgroundWorkerSkipReason(err), "timeout", timeout, "duration", time.Since(startedAt))
			return
		}
		if item.ID == "" {
			continue
		}
		records, err := p.refreshProject(runCtx, item.ID)
		if err != nil {
			if reason := backgroundWorkerSkipReason(err); reason != "" {
				skipped++
				slog.Info("application catalog projection project skipped", "projectID", item.ID, "reason", reason, "timeout", timeout, "duration", time.Since(startedAt))
				continue
			}
			failed++
			slog.Warn("application catalog projection refresh failed", "projectID", item.ID, "duration", time.Since(startedAt), "error", err)
			continue
		}
		refreshed++
		slog.Debug("application catalog projection refreshed", "projectID", item.ID, "applications", len(records))
	}
	slog.Debug("application catalog projection completed", "projects", len(projects), "refreshed", refreshed, "skipped", skipped, "failed", failed, "duration", time.Since(startedAt))
}

func (p *ApplicationCatalogProjector) refreshProject(ctx context.Context, projectID string) ([]Record, error) {
	if p.Store == nil {
		return nil, nil
	}
	return p.Store.RefreshProject(ctx, projectID)
}

func (p *ApplicationCatalogProjector) effectiveTimeout() time.Duration {
	if p == nil || p.Timeout <= 0 {
		return defaultApplicationCatalogProjectorTimeout
	}
	return p.Timeout
}
