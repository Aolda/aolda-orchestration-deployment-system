package kubernetes

import (
	"context"
	"time"

	"github.com/aolda/aods-backend/internal/application"
	"github.com/aolda/aods-backend/internal/core"
)

type LocalSyncStatusReader struct{}
type LocalNetworkExposureReader struct{}

func (LocalSyncStatusReader) Read(ctx context.Context, record application.Record) (application.SyncInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.SyncInfo{}, err
	}

	observedAt := record.UpdatedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	return application.SyncInfo{
		Status:     application.SyncStatusUnknown,
		Message:    "Kubernetes/Flux 연동이 설정되지 않아 동기화 상태를 확인할 수 없습니다.",
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

func (LocalNetworkExposureReader) Read(ctx context.Context, record application.Record) (application.NetworkExposureInfo, error) {
	if err := ctx.Err(); err != nil {
		return application.NetworkExposureInfo{}, err
	}

	observedAt := record.UpdatedAt
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}

	if !record.LoadBalancerEnabled {
		return application.NetworkExposureInfo{
			Status:      application.NetworkExposureStatusInternal,
			Message:     "현재는 내부 전용(ClusterIP) 서비스로 운영 중입니다.",
			ServiceType: "ClusterIP",
			ObservedAt:  observedAt,
		}, nil
	}

	return application.NetworkExposureInfo{
		Status:      application.NetworkExposureStatusPending,
		Message:     "Kubernetes API 연동이 설정되지 않아 실제 LoadBalancer 준비 상태를 조회할 수 없습니다. 현재는 요청 저장 상태만 확인할 수 있습니다.",
		ServiceType: "LoadBalancer",
		ObservedAt:  observedAt,
	}, nil
}

func NewNetworkExposureReader(cfg core.Config) application.NetworkExposureReader {
	if !cfg.UseKubernetesAPI() {
		return LocalNetworkExposureReader{}
	}

	reader, err := NewServiceNetworkExposureReader(cfg)
	if err != nil {
		return ErrorNetworkExposureReader{Err: err}
	}
	return reader
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

	return application.RolloutInfo{}, application.ValidationError{
		Message: "rollout integration is not configured",
		Details: map[string]any{
			"applicationId": record.ID,
			"mode":          "local",
		},
	}
}

func (LocalRolloutController) Promote(ctx context.Context, record application.Record, full bool) (application.RolloutInfo, error) {
	return LocalRolloutController{}.GetRollout(ctx, record)
}

func (LocalRolloutController) Abort(ctx context.Context, record application.Record) (application.RolloutInfo, error) {
	return LocalRolloutController{}.GetRollout(ctx, record)
}
