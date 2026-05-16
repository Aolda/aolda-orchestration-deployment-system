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
}

func (p *ApplicationCatalogProjector) Start(ctx context.Context) {
	if p == nil || p.Store == nil || p.Projects == nil {
		return
	}
	interval := p.Interval
	if interval <= 0 {
		return
	}

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
	projects, err := p.Projects.ListProjects(ctx)
	if err != nil {
		slog.Warn("application catalog projection project list failed", "error", err)
		return
	}
	for _, item := range projects {
		if item.ID == "" {
			continue
		}
		records, err := p.Store.RefreshProject(ctx, item.ID)
		if err != nil {
			slog.Warn("application catalog projection refresh failed", "projectID", item.ID, "error", err)
			continue
		}
		slog.Debug("application catalog projection refreshed", "projectID", item.ID, "applications", len(records))
	}
}
