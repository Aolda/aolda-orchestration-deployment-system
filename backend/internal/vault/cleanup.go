package vault

import (
	"context"
	"log/slog"
	"time"
)

const pendingCommitStatus = "pending_commit"

type StagingSecretCleaner interface {
	CleanupStale(ctx context.Context, cutoff time.Time) (int, error)
}

type StagingSecretCleanupWorker struct {
	Cleaner  StagingSecretCleaner
	Interval time.Duration
	MaxAge   time.Duration
}

func (w *StagingSecretCleanupWorker) Start(ctx context.Context) {
	if w == nil || w.Cleaner == nil || w.Interval <= 0 || w.MaxAge <= 0 {
		return
	}

	slog.Info(
		"starting stale staging secret cleanup worker",
		"interval", w.Interval,
		"maxAge", w.MaxAge,
	)

	w.runOnce(ctx)

	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *StagingSecretCleanupWorker) runOnce(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-w.MaxAge)
	count, err := w.Cleaner.CleanupStale(ctx, cutoff)
	if err != nil {
		slog.Error("stale staging secret cleanup failed", "error", err)
		return
	}
	if count > 0 {
		slog.Info("stale staging secrets cleaned up", "count", count, "cutoff", cutoff)
	}
}
