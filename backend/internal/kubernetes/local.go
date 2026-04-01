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
		Message:    "로컬 어댑터 기준으로 현재 워크스페이스 상태가 이미 반영된 것으로 판단했습니다.",
		ObservedAt: observedAt,
	}, nil
}
