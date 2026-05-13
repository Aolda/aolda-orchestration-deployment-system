package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/aolda/aods-backend/internal/project"
)

type DeploymentOperationStore interface {
	EnqueueDeploymentOperation(ctx context.Context, operation DeploymentOperation) (DeploymentOperation, error)
	ListDeploymentOperationRecords(ctx context.Context, applicationID string) ([]DeploymentRecord, error)
	GetDeploymentOperationRecord(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error)
	ClaimNextDeploymentOperation(ctx context.Context, workerID string, leaseDuration time.Duration) (DeploymentOperation, bool, error)
	MarkDeploymentOperationSucceeded(ctx context.Context, operation DeploymentOperation) error
	MarkDeploymentOperationRetry(ctx context.Context, operation DeploymentOperation, message string, nextAttemptAt time.Time) error
	MarkDeploymentOperationFailed(ctx context.Context, operation DeploymentOperation, message string) error
}

type DeploymentOperationWorker struct {
	Service       *Service
	Store         DeploymentOperationStore
	WorkerID      string
	Interval      time.Duration
	LeaseDuration time.Duration
}

func (w *DeploymentOperationWorker) Start(ctx context.Context) {
	if w == nil || w.Service == nil || w.Store == nil {
		return
	}
	workerID := strings.TrimSpace(w.WorkerID)
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%d", timeNowUTC().UnixNano())
	}
	interval := w.Interval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	leaseDuration := w.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = 5 * time.Minute
	}

	slog.Info("starting deployment operation worker", "workerId", workerID, "interval", interval, "leaseDuration", leaseDuration)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		for {
			processed, err := w.processOnce(ctx, workerID, leaseDuration)
			if err != nil {
				slog.Error("deployment operation worker failed", "workerId", workerID, "error", err)
				break
			}
			if !processed {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *DeploymentOperationWorker) processOnce(ctx context.Context, workerID string, leaseDuration time.Duration) (bool, error) {
	operation, ok, err := w.Store.ClaimNextDeploymentOperation(ctx, workerID, leaseDuration)
	if err != nil || !ok {
		return ok, err
	}

	response, runErr := w.Service.executeDeploymentOperation(ctx, operation)
	if runErr == nil {
		if err := w.Store.MarkDeploymentOperationSucceeded(ctx, operation); err != nil {
			return true, err
		}
		slog.Info(
			"deployment operation completed",
			"deploymentId", response.DeploymentID,
			"applicationId", response.ApplicationID,
			"imageTag", response.ImageTag,
		)
		return true, nil
	}

	message := runErr.Error()
	if !retryableDeploymentOperationError(runErr) || operation.AttemptCount >= maxDeploymentOperationAttempts(operation) {
		if err := w.Store.MarkDeploymentOperationFailed(ctx, operation, message); err != nil {
			return true, err
		}
		slog.Error(
			"deployment operation failed permanently",
			"deploymentId", operation.ID,
			"applicationId", operation.ApplicationID,
			"attempt", operation.AttemptCount,
			"error", runErr,
		)
		return true, nil
	}

	nextAttemptAt := timeNowUTC().Add(deploymentOperationBackoff(operation.AttemptCount))
	if err := w.Store.MarkDeploymentOperationRetry(ctx, operation, message, nextAttemptAt); err != nil {
		return true, err
	}
	slog.Warn(
		"deployment operation scheduled for retry",
		"deploymentId", operation.ID,
		"applicationId", operation.ApplicationID,
		"attempt", operation.AttemptCount,
		"nextAttemptAt", nextAttemptAt,
		"error", runErr,
	)
	return true, nil
}

func (operation DeploymentOperation) deploymentRecord() DeploymentRecord {
	message := strings.TrimSpace(operation.Message)
	if message == "" {
		message = strings.TrimSpace(operation.LastError)
	}
	if message == "" {
		switch operation.Status {
		case DeploymentOperationQueued:
			message = "배포 요청이 durable queue에 저장되었고 worker 실행을 기다리고 있습니다."
		case DeploymentOperationRunning:
			message = "배포 worker가 Git desired state 반영을 처리하고 있습니다."
		case DeploymentOperationRetrying:
			message = "외부 의존성 또는 Git write 실패 후 재시도를 기다리고 있습니다."
		case DeploymentOperationFailed:
			message = "배포 operation이 재시도 한도를 초과했습니다."
		}
	}
	return DeploymentRecord{
		DeploymentID:       operation.ID,
		ApplicationID:      operation.ApplicationID,
		ProjectID:          operation.ProjectID,
		ApplicationName:    operation.ApplicationName,
		Environment:        operation.Environment,
		Image:              operation.DesiredImage,
		ImageTag:           operation.ImageTag,
		DeploymentStrategy: operation.DeploymentStrategy,
		Status:             string(operation.Status),
		Message:            message,
		CreatedAt:          operation.CreatedAt,
		UpdatedAt:          operation.UpdatedAt,
	}
}

func mergeDeploymentRecords(primary []DeploymentRecord, operationRecords []DeploymentRecord) []DeploymentRecord {
	if len(operationRecords) == 0 {
		return primary
	}
	seen := make(map[string]struct{}, len(primary)+len(operationRecords))
	items := make([]DeploymentRecord, 0, len(primary)+len(operationRecords))
	for _, item := range primary {
		seen[item.DeploymentID] = struct{}{}
		items = append(items, item)
	}
	for _, item := range operationRecords {
		if _, ok := seen[item.DeploymentID]; ok {
			continue
		}
		seen[item.DeploymentID] = struct{}{}
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

func maxDeploymentOperationAttempts(operation DeploymentOperation) int {
	if operation.MaxAttempts <= 0 {
		return 5
	}
	return operation.MaxAttempts
}

func deploymentOperationBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return time.Minute
	}
	delay := time.Minute
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= 15*time.Minute {
			return 15 * time.Minute
		}
	}
	return delay
}

func retryableDeploymentOperationError(err error) bool {
	if err == nil {
		return false
	}
	var validation ValidationError
	var image ImageValidationError
	switch {
	case errors.As(err, &validation):
		return false
	case errors.As(err, &image):
		return image.Code == "IMAGE_CHECK_FAILED"
	case errors.Is(err, ErrInvalidID),
		errors.Is(err, ErrNotFound),
		errors.Is(err, ErrArchived),
		errors.Is(err, ErrChangeRequired),
		errors.Is(err, ErrInvalidPolicy),
		errors.Is(err, project.ErrNotFound),
		errors.Is(err, project.ErrForbidden):
		return false
	default:
		return true
	}
}
