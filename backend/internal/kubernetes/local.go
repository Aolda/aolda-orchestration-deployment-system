package kubernetes

import (
	"context"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
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

func (r LocalSyncStatusReader) ReadMany(ctx context.Context, records []application.Record) (map[string]application.SyncInfo, error) {
	items := make(map[string]application.SyncInfo, len(records))
	for _, record := range records {
		info, err := r.Read(ctx, record)
		if err != nil {
			return nil, err
		}
		items[record.ID] = info
	}
	return items, nil
}

type LocalRolloutController struct{}

func NewRolloutController(cfg core.Config) application.RolloutController {
	if !cfg.UseKubernetesAPI() {
		return LocalRolloutController{}
	}

	controller, err := NewArgoRolloutController(cfg)
	if err != nil {
		return ErrorRolloutController{Err: err}
	}
	return controller
}

func (LocalRolloutController) GetRollout(ctx context.Context, record application.Record) (application.RolloutInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.RolloutInfo{}, err
	}
	weight := 100
	step := 4
	return application.RolloutInfo{
		Phase:          "Healthy",
		CurrentStep:    &step,
		CanaryWeight:   &weight,
		StableRevision: "stable",
		CanaryRevision: record.Image,
		Message:        "로컬 롤아웃 어댑터 기준으로 현재 배포가 완료된 것으로 판단했습니다.",
	}, nil
}

func (LocalRolloutController) Promote(ctx context.Context, record application.Record, full bool) (application.RolloutInfo, error) {
	return LocalRolloutController{}.GetRollout(ctx, record)
}

func (LocalRolloutController) Abort(ctx context.Context, record application.Record) (application.RolloutInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.RolloutInfo{}, err
	}
	weight := 0
	return application.RolloutInfo{
		Phase:          "Degraded",
		CanaryWeight:   &weight,
		StableRevision: "stable",
		CanaryRevision: record.Image,
		Message:        "로컬 롤아웃 어댑터 기준으로 중단 요청을 반영했습니다.",
	}, nil
}
