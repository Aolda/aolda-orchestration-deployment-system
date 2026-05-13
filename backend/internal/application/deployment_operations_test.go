package application

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/aolda/aods-backend/internal/project"
)

type workerOperationStore struct {
	operation DeploymentOperation
	claimOK   bool
	claimErr  error

	claimedWorker string
	claimedLease  time.Duration
	succeeded     []DeploymentOperation
	retries       []operationRetryCall
	failures      []operationFailureCall
}

type operationRetryCall struct {
	operation     DeploymentOperation
	message       string
	nextAttemptAt time.Time
}

type operationFailureCall struct {
	operation DeploymentOperation
	message   string
}

func (s *workerOperationStore) EnqueueDeploymentOperation(context.Context, DeploymentOperation) (DeploymentOperation, error) {
	return DeploymentOperation{}, nil
}

func (s *workerOperationStore) ListDeploymentOperationRecords(context.Context, string) ([]DeploymentRecord, error) {
	return nil, nil
}

func (s *workerOperationStore) GetDeploymentOperationRecord(context.Context, string, string) (DeploymentRecord, error) {
	return DeploymentRecord{}, ErrDeploymentNotFound
}

func (s *workerOperationStore) ClaimNextDeploymentOperation(ctx context.Context, workerID string, leaseDuration time.Duration) (DeploymentOperation, bool, error) {
	s.claimedWorker = workerID
	s.claimedLease = leaseDuration
	return s.operation, s.claimOK, s.claimErr
}

func (s *workerOperationStore) MarkDeploymentOperationSucceeded(ctx context.Context, operation DeploymentOperation) error {
	s.succeeded = append(s.succeeded, operation)
	return nil
}

func (s *workerOperationStore) MarkDeploymentOperationRetry(ctx context.Context, operation DeploymentOperation, message string, nextAttemptAt time.Time) error {
	s.retries = append(s.retries, operationRetryCall{operation: operation, message: message, nextAttemptAt: nextAttemptAt})
	return nil
}

func (s *workerOperationStore) MarkDeploymentOperationFailed(ctx context.Context, operation DeploymentOperation, message string) error {
	s.failures = append(s.failures, operationFailureCall{operation: operation, message: message})
	return nil
}

func TestDeploymentOperationWorkerProcessOnceSucceedsAndMarksOperation(t *testing.T) {
	t.Parallel()

	record := deploymentOperationTestRecord()
	mutations := &deploymentMutationStore{stubStore: stubStore{records: []Record{record}}}
	operationStore := &workerOperationStore{
		operation: deploymentOperationTestOperation(record),
		claimOK:   true,
	}
	service := Service{
		Projects: authorizedProjectService(),
		Store:    mutations,
		Images:   NoopImageVerifier{},
	}
	worker := DeploymentOperationWorker{Service: &service, Store: operationStore}

	processed, err := worker.processOnce(context.Background(), "worker-a", 90*time.Second)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected operation to be processed")
	}
	if operationStore.claimedWorker != "worker-a" || operationStore.claimedLease != 90*time.Second {
		t.Fatalf("unexpected claim arguments: worker=%q lease=%s", operationStore.claimedWorker, operationStore.claimedLease)
	}
	if mutations.updateImageCalls != 1 {
		t.Fatalf("expected one desired-state image update, got %d", mutations.updateImageCalls)
	}
	if len(operationStore.succeeded) != 1 {
		t.Fatalf("expected success mark, got %d", len(operationStore.succeeded))
	}
	if operationStore.succeeded[0].ID != "dep_worker" {
		t.Fatalf("unexpected succeeded operation: %#v", operationStore.succeeded[0])
	}
	if len(operationStore.retries) != 0 || len(operationStore.failures) != 0 {
		t.Fatalf("expected no retry/failure marks, got retries=%d failures=%d", len(operationStore.retries), len(operationStore.failures))
	}
}

func TestDeploymentOperationWorkerRetriesTransientErrors(t *testing.T) {
	t.Parallel()

	record := deploymentOperationTestRecord()
	operation := deploymentOperationTestOperation(record)
	operation.AttemptCount = 1
	operation.MaxAttempts = 3
	operationStore := &workerOperationStore{
		operation: operation,
		claimOK:   true,
	}
	service := Service{
		Projects: authorizedProjectService(),
		Store:    &deploymentMutationStore{stubStore: stubStore{records: []Record{record}}},
		Images:   failingImageVerifier{err: errors.New("registry timeout")},
	}
	worker := DeploymentOperationWorker{Service: &service, Store: operationStore}

	processed, err := worker.processOnce(context.Background(), "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected operation to be processed")
	}
	if len(operationStore.retries) != 1 {
		t.Fatalf("expected retry mark, got %d", len(operationStore.retries))
	}
	if !strings.Contains(operationStore.retries[0].message, "registry timeout") {
		t.Fatalf("expected retry message to preserve cause, got %q", operationStore.retries[0].message)
	}
	if !operationStore.retries[0].nextAttemptAt.After(timeNowUTC()) {
		t.Fatalf("expected retry to be scheduled in the future, got %s", operationStore.retries[0].nextAttemptAt)
	}
	if len(operationStore.succeeded) != 0 || len(operationStore.failures) != 0 {
		t.Fatalf("expected only retry mark, got success=%d failures=%d", len(operationStore.succeeded), len(operationStore.failures))
	}
}

func TestDeploymentOperationWorkerFailsNonRetryableErrors(t *testing.T) {
	t.Parallel()

	record := deploymentOperationTestRecord()
	operationStore := &workerOperationStore{
		operation: deploymentOperationTestOperation(record),
		claimOK:   true,
	}
	service := Service{
		Projects: authorizedProjectService(),
		Store:    &deploymentMutationStore{stubStore: stubStore{records: []Record{record}}},
		Images: failingImageVerifier{err: ImageValidationError{
			Code:    "IMAGE_NOT_FOUND",
			Message: "image tag was not found",
			Image:   "ghcr.io/example/demo:v2",
		}},
	}
	worker := DeploymentOperationWorker{Service: &service, Store: operationStore}

	processed, err := worker.processOnce(context.Background(), "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected operation to be processed")
	}
	if len(operationStore.failures) != 1 {
		t.Fatalf("expected permanent failure mark, got %d", len(operationStore.failures))
	}
	if !strings.Contains(operationStore.failures[0].message, "image tag was not found") {
		t.Fatalf("expected failure message to preserve cause, got %q", operationStore.failures[0].message)
	}
	if len(operationStore.succeeded) != 0 || len(operationStore.retries) != 0 {
		t.Fatalf("expected only failure mark, got success=%d retries=%d", len(operationStore.succeeded), len(operationStore.retries))
	}
}

func TestDeploymentOperationWorkerFailsWhenRetryBudgetIsExhausted(t *testing.T) {
	t.Parallel()

	record := deploymentOperationTestRecord()
	operation := deploymentOperationTestOperation(record)
	operation.AttemptCount = 3
	operation.MaxAttempts = 3
	operationStore := &workerOperationStore{operation: operation, claimOK: true}
	service := Service{
		Projects: authorizedProjectService(),
		Store:    &deploymentMutationStore{stubStore: stubStore{records: []Record{record}}},
		Images:   failingImageVerifier{err: errors.New("git push failed")},
	}
	worker := DeploymentOperationWorker{Service: &service, Store: operationStore}

	processed, err := worker.processOnce(context.Background(), "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("process once: %v", err)
	}
	if !processed {
		t.Fatal("expected operation to be processed")
	}
	if len(operationStore.failures) != 1 {
		t.Fatalf("expected permanent failure after retry budget, got %d", len(operationStore.failures))
	}
	if len(operationStore.retries) != 0 {
		t.Fatalf("expected no retry after budget exhaustion, got %d", len(operationStore.retries))
	}
}

func TestDeploymentOperationWorkerPropagatesClaimError(t *testing.T) {
	t.Parallel()

	operationStore := &workerOperationStore{claimErr: errors.New("database unavailable")}
	worker := DeploymentOperationWorker{
		Service: &Service{},
		Store:   operationStore,
	}

	processed, err := worker.processOnce(context.Background(), "worker-a", time.Minute)
	if err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("expected claim error, got %v", err)
	}
	if processed {
		t.Fatal("expected no processed operation on claim error")
	}
}

func TestDeploymentOperationRecordUsesDefaultMessages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status  DeploymentOperationStatus
		message string
	}{
		{status: DeploymentOperationQueued, message: "worker 실행을 기다리고 있습니다"},
		{status: DeploymentOperationRunning, message: "Git desired state 반영을 처리하고 있습니다"},
		{status: DeploymentOperationRetrying, message: "재시도를 기다리고 있습니다"},
		{status: DeploymentOperationFailed, message: "재시도 한도를 초과했습니다"},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()

			record := DeploymentOperation{
				ID:                 "dep_1",
				ApplicationID:      "project-a__demo",
				ProjectID:          "project-a",
				ApplicationName:    "demo",
				Environment:        "shared",
				DesiredImage:       "ghcr.io/example/demo:v2",
				ImageTag:           "v2",
				DeploymentStrategy: DeploymentStrategyRollout,
				Status:             tt.status,
			}.deploymentRecord()

			if record.Status != string(tt.status) {
				t.Fatalf("expected status %s, got %s", tt.status, record.Status)
			}
			if !strings.Contains(record.Message, tt.message) {
				t.Fatalf("expected default message containing %q, got %q", tt.message, record.Message)
			}
		})
	}
}

func TestDeploymentOperationRecordPrefersMessageThenLastError(t *testing.T) {
	t.Parallel()

	withMessage := DeploymentOperation{
		ID:      "dep_1",
		Status:  DeploymentOperationRetrying,
		Message: "custom message",
	}.deploymentRecord()
	if withMessage.Message != "custom message" {
		t.Fatalf("expected explicit message, got %q", withMessage.Message)
	}

	withLastError := DeploymentOperation{
		ID:        "dep_2",
		Status:    DeploymentOperationRetrying,
		LastError: "last error",
	}.deploymentRecord()
	if withLastError.Message != "last error" {
		t.Fatalf("expected last error fallback, got %q", withLastError.Message)
	}
}

func TestMergeDeploymentRecordsSortsAndDeduplicates(t *testing.T) {
	t.Parallel()

	older := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	primary := []DeploymentRecord{
		{DeploymentID: "dep_existing", CreatedAt: older},
		{DeploymentID: "dep_duplicate", CreatedAt: older.Add(30 * time.Minute), Status: "Completed"},
	}
	operations := []DeploymentRecord{
		{DeploymentID: "dep_queued", CreatedAt: newer, Status: string(DeploymentOperationQueued)},
		{DeploymentID: "dep_duplicate", CreatedAt: newer, Status: string(DeploymentOperationRetrying)},
	}

	items := mergeDeploymentRecords(primary, operations)
	if len(items) != 3 {
		t.Fatalf("expected 3 merged records, got %d", len(items))
	}
	if items[0].DeploymentID != "dep_queued" {
		t.Fatalf("expected newest queued operation first, got %#v", items[0])
	}
	for _, item := range items {
		if item.DeploymentID == "dep_duplicate" && item.Status != "Completed" {
			t.Fatalf("expected primary record to win duplicate merge, got %#v", item)
		}
	}
}

func TestDeploymentOperationBackoffAndRetryability(t *testing.T) {
	t.Parallel()

	backoffCases := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 0, want: time.Minute},
		{attempt: 1, want: time.Minute},
		{attempt: 2, want: 2 * time.Minute},
		{attempt: 5, want: 15 * time.Minute},
	}
	for _, tc := range backoffCases {
		if got := deploymentOperationBackoff(tc.attempt); got != tc.want {
			t.Fatalf("attempt %d backoff: got %s want %s", tc.attempt, got, tc.want)
		}
	}

	if maxDeploymentOperationAttempts(DeploymentOperation{}) != 5 {
		t.Fatal("expected default max attempts to be 5")
	}
	if maxDeploymentOperationAttempts(DeploymentOperation{MaxAttempts: 9}) != 9 {
		t.Fatal("expected configured max attempts to be used")
	}

	retryCases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "validation", err: ValidationError{Message: "bad request"}, want: false},
		{name: "image check failed", err: ImageValidationError{Code: "IMAGE_CHECK_FAILED", Message: "registry unavailable"}, want: true},
		{name: "image not found", err: ImageValidationError{Code: "IMAGE_NOT_FOUND", Message: "missing"}, want: false},
		{name: "application not found", err: ErrNotFound, want: false},
		{name: "project forbidden", err: project.ErrForbidden, want: false},
		{name: "generic dependency", err: errors.New("git push failed"), want: true},
	}
	for _, tc := range retryCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := retryableDeploymentOperationError(tc.err); got != tc.want {
				t.Fatalf("retryable=%v want %v for %v", got, tc.want, tc.err)
			}
		})
	}
}

func TestMariaDBDeploymentOperationStoreNilDatabaseErrors(t *testing.T) {
	t.Parallel()

	store := MariaDBDeploymentOperationStore{}
	ctx := context.Background()
	operation := DeploymentOperation{ID: "dep_1"}

	if err := store.Ensure(ctx); err == nil {
		t.Fatal("expected Ensure to fail without database")
	}
	if _, err := store.EnqueueDeploymentOperation(ctx, operation); err == nil {
		t.Fatal("expected enqueue to fail without database")
	}
	if _, err := store.ListDeploymentOperationRecords(ctx, "app"); err == nil {
		t.Fatal("expected list to fail without database")
	}
	if _, err := store.GetDeploymentOperationRecord(ctx, "app", "dep_1"); err == nil {
		t.Fatal("expected get to fail without database")
	}
	if _, _, err := store.ClaimNextDeploymentOperation(ctx, "worker", time.Minute); err == nil {
		t.Fatal("expected claim to fail without database")
	}
	if err := store.MarkDeploymentOperationRetry(ctx, operation, "retry", time.Now()); err == nil {
		t.Fatal("expected retry mark to fail without database")
	}
	if err := store.MarkDeploymentOperationSucceeded(ctx, operation); err == nil {
		t.Fatal("expected success mark to fail without database")
	}
	if err := store.MarkDeploymentOperationFailed(ctx, operation, "failed"); err == nil {
		t.Fatal("expected failure mark to fail without database")
	}
}

func TestMariaDBDeploymentOperationStoreEnsureCreatesTables(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newDeploymentOperationSQLMock(t)
	defer cleanup()

	mock.ExpectExec("(?s)CREATE TABLE IF NOT EXISTS aods_deployment_operations.*").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("(?s)CREATE TABLE IF NOT EXISTS aods_operation_locks.*").
		WillReturnResult(sqlmock.NewResult(0, 0))

	store := MariaDBDeploymentOperationStore{DB: db}
	if err := store.Ensure(context.Background()); err != nil {
		t.Fatalf("ensure store: %v", err)
	}
	assertDeploymentOperationSQLExpectations(t, mock)
}

func TestMariaDBDeploymentOperationStoreEnqueueDefaultsAndReadsBack(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newDeploymentOperationSQLMock(t)
	defer cleanup()

	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	input := deploymentOperationTestOperation(deploymentOperationTestRecord())
	input.Status = ""
	input.Message = "  queued  "
	input.AttemptCount = 0
	input.MaxAttempts = 0
	input.NextAttemptAt = now
	input.CreatedAt = now
	input.UpdatedAt = time.Time{}
	input.Version = 0
	stored := input
	stored.Status = DeploymentOperationQueued
	stored.Message = "queued"
	stored.MaxAttempts = 5
	stored.UpdatedAt = now.Add(time.Second)
	stored.Version = 1

	mock.ExpectExec("(?s)INSERT INTO aods_deployment_operations.*").
		WithArgs(
			input.ID,
			input.ApplicationID,
			input.ProjectID,
			input.ApplicationName,
			input.Environment,
			input.ImageTag,
			input.DesiredImage,
			string(input.DeploymentStrategy),
			input.RequestedBy,
			input.RequestID,
			input.LockKey,
			string(DeploymentOperationQueued),
			"queued",
			input.AttemptCount,
			5,
			nil,
			input.NextAttemptAt,
			input.LeaseOwner,
			input.LeaseUntil,
			int64(1),
			input.CreatedAt,
			sqlmock.AnyArg(),
			nil,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	expectGetDeploymentOperation(mock, stored)

	store := MariaDBDeploymentOperationStore{DB: db}
	operation, err := store.EnqueueDeploymentOperation(context.Background(), input)
	if err != nil {
		t.Fatalf("enqueue operation: %v", err)
	}
	if operation.Status != DeploymentOperationQueued || operation.Message != "queued" || operation.MaxAttempts != 5 {
		t.Fatalf("unexpected stored operation defaults: %#v", operation)
	}
	assertDeploymentOperationSQLExpectations(t, mock)
}

func TestMariaDBDeploymentOperationStoreListAndGetRecords(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newDeploymentOperationSQLMock(t)
	defer cleanup()

	queued := deploymentOperationTestOperation(deploymentOperationTestRecord())
	queued.Status = DeploymentOperationQueued
	queued.Message = ""
	retrying := queued
	retrying.ID = "dep_retry"
	retrying.Status = DeploymentOperationRetrying
	retrying.LastError = "git push failed"

	mock.ExpectQuery("(?s)SELECT .*FROM aods_deployment_operations\\s+WHERE application_id = .*ORDER BY created_at DESC").
		WithArgs(queued.ApplicationID, string(DeploymentOperationCompleted)).
		WillReturnRows(deploymentOperationRows(queued, retrying))
	mock.ExpectQuery("(?s)SELECT .*FROM aods_deployment_operations\\s+WHERE id = .*AND application_id = .*").
		WithArgs("dep_missing", queued.ApplicationID).
		WillReturnRows(deploymentOperationRows())

	store := MariaDBDeploymentOperationStore{DB: db}
	items, err := store.ListDeploymentOperationRecords(context.Background(), queued.ApplicationID)
	if err != nil {
		t.Fatalf("list operation records: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two operation records, got %d", len(items))
	}
	if items[0].Status != string(DeploymentOperationQueued) || !strings.Contains(items[0].Message, "worker 실행") {
		t.Fatalf("expected queued record default message, got %#v", items[0])
	}
	if items[1].Message != "git push failed" {
		t.Fatalf("expected last error fallback message, got %q", items[1].Message)
	}

	if _, err := store.GetDeploymentOperationRecord(context.Background(), queued.ApplicationID, "dep_missing"); !errors.Is(err, ErrDeploymentNotFound) {
		t.Fatalf("expected deployment not found, got %v", err)
	}
	assertDeploymentOperationSQLExpectations(t, mock)
}

func TestMariaDBDeploymentOperationStoreClaimNextOperation(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newDeploymentOperationSQLMock(t)
	defer cleanup()

	operation := deploymentOperationTestOperation(deploymentOperationTestRecord())
	operation.Status = DeploymentOperationQueued
	operation.AttemptCount = 0
	operation.LeaseOwner = ""
	operation.LeaseUntil = time.Time{}
	operation.Version = 4

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)SELECT .*FROM aods_deployment_operations\\s+WHERE status IN .*FOR UPDATE SKIP LOCKED").
		WithArgs(string(DeploymentOperationQueued), string(DeploymentOperationRetrying), sqlmock.AnyArg()).
		WillReturnRows(deploymentOperationRows(operation))
	mock.ExpectExec("INSERT IGNORE INTO aods_operation_locks").
		WithArgs(operation.LockKey, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT lease_owner, lease_until, version FROM aods_operation_locks WHERE lock_key = .*FOR UPDATE").
		WithArgs(operation.LockKey).
		WillReturnRows(sqlmock.NewRows([]string{"lease_owner", "lease_until", "version"}).
			AddRow("", time.Unix(0, 0).UTC(), int64(2)))
	mock.ExpectExec("(?s)UPDATE aods_operation_locks\\s+SET lease_owner = .*").
		WithArgs("worker-a", sqlmock.AnyArg(), sqlmock.AnyArg(), operation.LockKey, int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)UPDATE aods_deployment_operations\\s+SET status = .*WHERE id = .*AND version = .*").
		WithArgs(
			string(DeploymentOperationRunning),
			"배포 worker가 Git desired state 반영을 처리하고 있습니다.",
			1,
			"worker-a",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			operation.ID,
			operation.Version,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	store := MariaDBDeploymentOperationStore{DB: db}
	claimed, ok, err := store.ClaimNextDeploymentOperation(context.Background(), "worker-a", 3*time.Minute)
	if err != nil {
		t.Fatalf("claim next operation: %v", err)
	}
	if !ok {
		t.Fatal("expected operation to be claimed")
	}
	if claimed.Status != DeploymentOperationRunning || claimed.AttemptCount != 1 || claimed.LeaseOwner != "worker-a" {
		t.Fatalf("unexpected claimed operation: %#v", claimed)
	}
	if claimed.Version != operation.Version+1 {
		t.Fatalf("expected claimed version to increment, got %d", claimed.Version)
	}
	assertDeploymentOperationSQLExpectations(t, mock)
}

func TestMariaDBDeploymentOperationStoreMarkRetryAndTerminalStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(t *testing.T, store MariaDBDeploymentOperationStore, operation DeploymentOperation)
	}{
		{
			name: "retry",
			run: func(t *testing.T, store MariaDBDeploymentOperationStore, operation DeploymentOperation) {
				t.Helper()
				if err := store.MarkDeploymentOperationRetry(context.Background(), operation, "registry timeout", time.Date(2026, 5, 12, 12, 5, 0, 0, time.UTC)); err != nil {
					t.Fatalf("mark retry: %v", err)
				}
			},
		},
		{
			name: "succeeded",
			run: func(t *testing.T, store MariaDBDeploymentOperationStore, operation DeploymentOperation) {
				t.Helper()
				if err := store.MarkDeploymentOperationSucceeded(context.Background(), operation); err != nil {
					t.Fatalf("mark succeeded: %v", err)
				}
			},
		},
		{
			name: "failed",
			run: func(t *testing.T, store MariaDBDeploymentOperationStore, operation DeploymentOperation) {
				t.Helper()
				if err := store.MarkDeploymentOperationFailed(context.Background(), operation, "git push failed"); err != nil {
					t.Fatalf("mark failed: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock, cleanup := newDeploymentOperationSQLMock(t)
			defer cleanup()

			operation := deploymentOperationTestOperation(deploymentOperationTestRecord())
			operation.LeaseOwner = "worker-a"

			mock.ExpectBegin()
			mock.ExpectExec("(?s)UPDATE aods_operation_locks\\s+SET lease_owner = '', lease_until = .*").
				WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), operation.LockKey, operation.LeaseOwner).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("(?s)UPDATE aods_deployment_operations\\s+SET status = .*WHERE id = .*AND lease_owner = .*").
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			store := MariaDBDeploymentOperationStore{DB: db}
			tt.run(t, store, operation)
			assertDeploymentOperationSQLExpectations(t, mock)
		})
	}
}

type deploymentOperationScannerStub struct {
	values []any
	err    error
}

func (s deploymentOperationScannerStub) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	for index, value := range s.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *int:
			*target = value.(int)
		case *int64:
			*target = value.(int64)
		case *time.Time:
			*target = value.(time.Time)
		case *sql.NullString:
			*target = value.(sql.NullString)
		case *sql.NullTime:
			*target = value.(sql.NullTime)
		default:
			panic("unsupported scan destination")
		}
	}
	return nil
}

func TestScanDeploymentOperationMapsNullableColumns(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	completedAt := now.Add(time.Minute)
	operation, err := scanDeploymentOperation(deploymentOperationScannerStub{values: []any{
		"dep_1",
		"project-a__demo",
		"project-a",
		"demo",
		"shared",
		"v2",
		"ghcr.io/example/demo:v2",
		string(DeploymentStrategyRollout),
		"deployer",
		"req_1",
		"git:unit",
		string(DeploymentOperationCompleted),
		sql.NullString{String: "done", Valid: true},
		2,
		5,
		sql.NullString{String: "last", Valid: true},
		now,
		sql.NullString{String: "worker-a", Valid: true},
		sql.NullTime{Time: now.Add(5 * time.Minute), Valid: true},
		int64(7),
		now.Add(-time.Hour),
		now,
		sql.NullTime{Time: completedAt, Valid: true},
	}})
	if err != nil {
		t.Fatalf("scan operation: %v", err)
	}
	if operation.ID != "dep_1" || operation.DeploymentStrategy != DeploymentStrategyRollout || operation.Status != DeploymentOperationCompleted {
		t.Fatalf("unexpected operation identity fields: %#v", operation)
	}
	if operation.Message != "done" || operation.LastError != "last" || operation.LeaseOwner != "worker-a" {
		t.Fatalf("unexpected nullable strings: %#v", operation)
	}
	if operation.CompletedAt == nil || !operation.CompletedAt.Equal(completedAt) {
		t.Fatalf("expected completedAt %s, got %v", completedAt, operation.CompletedAt)
	}

	_, err = scanDeploymentOperation(deploymentOperationScannerStub{err: errors.New("scan failed")})
	if err == nil || !strings.Contains(err.Error(), "scan failed") {
		t.Fatalf("expected scan error, got %v", err)
	}
}

func TestMariaDBDeploymentOperationHelpers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 12, 13, 0, 0, 0, time.UTC)
	if nullableString("   ") != nil {
		t.Fatal("expected blank nullable string to become nil")
	}
	if nullableString("value") != "value" {
		t.Fatal("expected nonblank nullable string to pass through")
	}
	if nullableTime(time.Time{}) != nil {
		t.Fatal("expected zero nullable time to become nil")
	}
	if nullableTime(now) != now {
		t.Fatal("expected nonzero nullable time to pass through")
	}
	if nullableTimePtr(nil) != nil {
		t.Fatal("expected nil time pointer to become nil")
	}
	if nullableTimePtr(&now) != now {
		t.Fatal("expected nonzero time pointer to pass through")
	}
	if maxInt64(0, 1) != 1 || maxInt64(9, 1) != 9 {
		t.Fatal("expected maxInt64 to apply fallback only for nonpositive values")
	}
}

func deploymentOperationTestRecord() Record {
	return Record{
		ID:                 "project-a__demo",
		ProjectID:          "project-a",
		Name:               "demo",
		Image:              "ghcr.io/example/demo:v1",
		DeploymentStrategy: DeploymentStrategyRollout,
		DefaultEnvironment: "shared",
	}
}

func deploymentOperationTestOperation(record Record) DeploymentOperation {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)
	return DeploymentOperation{
		ID:                 "dep_worker",
		ApplicationID:      record.ID,
		ProjectID:          record.ProjectID,
		ApplicationName:    record.Name,
		Environment:        "shared",
		ImageTag:           "v2",
		DesiredImage:       "ghcr.io/example/demo:v2",
		DeploymentStrategy: record.DeploymentStrategy,
		RequestedBy:        "deployer",
		RequestID:          "req_worker",
		LockKey:            "git:unit",
		Status:             DeploymentOperationRunning,
		AttemptCount:       1,
		MaxAttempts:        3,
		LeaseOwner:         "worker-a",
		LeaseUntil:         now.Add(5 * time.Minute),
		Version:            2,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

func newDeploymentOperationSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, func()) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	return db, mock, func() {
		mock.ExpectClose()
		if err := db.Close(); err != nil {
			t.Fatalf("close sql mock db: %v", err)
		}
	}
}

func assertDeploymentOperationSQLExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func expectGetDeploymentOperation(mock sqlmock.Sqlmock, operation DeploymentOperation) {
	mock.ExpectQuery("(?s)SELECT .*FROM aods_deployment_operations WHERE id = .*").
		WithArgs(operation.ID).
		WillReturnRows(deploymentOperationRows(operation))
}

func deploymentOperationRows(operations ...DeploymentOperation) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{
		"id",
		"application_id",
		"project_id",
		"application_name",
		"environment",
		"image_tag",
		"desired_image",
		"deployment_strategy",
		"requested_by",
		"request_id",
		"lock_key",
		"status",
		"message",
		"attempt_count",
		"max_attempts",
		"last_error",
		"next_attempt_at",
		"lease_owner",
		"lease_until",
		"version",
		"created_at",
		"updated_at",
		"completed_at",
	})
	for _, operation := range operations {
		rows.AddRow(
			operation.ID,
			operation.ApplicationID,
			operation.ProjectID,
			operation.ApplicationName,
			operation.Environment,
			operation.ImageTag,
			operation.DesiredImage,
			string(operation.DeploymentStrategy),
			operation.RequestedBy,
			operation.RequestID,
			operation.LockKey,
			string(operation.Status),
			nullableString(operation.Message),
			operation.AttemptCount,
			operation.MaxAttempts,
			nullableString(operation.LastError),
			operation.NextAttemptAt,
			nullableString(operation.LeaseOwner),
			nullableTime(operation.LeaseUntil),
			operation.Version,
			operation.CreatedAt,
			operation.UpdatedAt,
			nullableTimePtr(operation.CompletedAt),
		)
	}
	return rows
}
