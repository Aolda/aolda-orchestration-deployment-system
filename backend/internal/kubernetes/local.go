package kubernetes

import (
	"context"
	"time"

	"github.com/aolda/aods-backend/internal/application"
)

type LocalSyncStatusReader struct{}

func (LocalSyncStatusReader) Read(ctx context.Context, record application.Record) (application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.SyncInfo{}, err
	}

	observedAt := record.UpdatedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	return application.SyncInfo{
		Status:     application.SyncStatusSynced,
		Message:    "Local adapter assumes the workspace desired state is already synced.",
		ObservedAt: observedAt,
	}, nil
}
