package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type MariaDBDeploymentOperationStore struct {
	DB *sql.DB
}

func (s MariaDBDeploymentOperationStore) Ensure(ctx context.Context) error {
	if s.DB == nil {
		return fmt.Errorf("mariadb deployment operation store is not configured")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS aods_deployment_operations (
			id VARCHAR(96) NOT NULL PRIMARY KEY,
			application_id VARCHAR(255) NOT NULL,
			project_id VARCHAR(128) NOT NULL,
			application_name VARCHAR(128) NOT NULL,
			environment VARCHAR(128) NOT NULL,
			image_tag VARCHAR(255) NOT NULL,
			desired_image TEXT NOT NULL,
			deployment_strategy VARCHAR(32) NOT NULL,
			requested_by VARCHAR(255) NOT NULL,
			request_id VARCHAR(128) NOT NULL,
			lock_key VARCHAR(255) NOT NULL,
			status VARCHAR(32) NOT NULL,
			message TEXT NULL,
			attempt_count INT NOT NULL DEFAULT 0,
			max_attempts INT NOT NULL DEFAULT 5,
			last_error TEXT NULL,
			next_attempt_at DATETIME(6) NOT NULL,
			lease_owner VARCHAR(255) NULL,
			lease_until DATETIME(6) NULL,
			version BIGINT NOT NULL DEFAULT 1,
			created_at DATETIME(6) NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			completed_at DATETIME(6) NULL,
			UNIQUE KEY uq_aods_deployment_operations_request_id (request_id),
			KEY idx_aods_deployment_operations_application (application_id, created_at),
			KEY idx_aods_deployment_operations_due (status, next_attempt_at, created_at),
			KEY idx_aods_deployment_operations_lock (lock_key, status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS aods_operation_locks (
			lock_key VARCHAR(255) NOT NULL PRIMARY KEY,
			lease_owner VARCHAR(255) NOT NULL,
			lease_until DATETIME(6) NOT NULL,
			version BIGINT NOT NULL DEFAULT 1,
			updated_at DATETIME(6) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, statement := range statements {
		if _, err := s.DB.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s MariaDBDeploymentOperationStore) EnqueueDeploymentOperation(ctx context.Context, operation DeploymentOperation) (DeploymentOperation, error) {
	if s.DB == nil {
		return DeploymentOperation{}, fmt.Errorf("mariadb deployment operation store is not configured")
	}
	now := timeNowUTC()
	if operation.CreatedAt.IsZero() {
		operation.CreatedAt = now
	}
	operation.UpdatedAt = now
	if operation.NextAttemptAt.IsZero() {
		operation.NextAttemptAt = now
	}
	if operation.Status == "" {
		operation.Status = DeploymentOperationQueued
	}
	if operation.MaxAttempts <= 0 {
		operation.MaxAttempts = 5
	}
	operation.Message = strings.TrimSpace(operation.Message)

	_, err := s.DB.ExecContext(
		ctx,
		`INSERT INTO aods_deployment_operations (
			id, application_id, project_id, application_name, environment, image_tag, desired_image,
			deployment_strategy, requested_by, request_id, lock_key, status, message, attempt_count,
			max_attempts, last_error, next_attempt_at, lease_owner, lease_until, version, created_at,
			updated_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE id = id`,
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
		maxInt64(operation.Version, 1),
		operation.CreatedAt,
		operation.UpdatedAt,
		nullableTimePtr(operation.CompletedAt),
	)
	if err != nil {
		return DeploymentOperation{}, err
	}
	return s.getOperation(ctx, operation.ID)
}

func (s MariaDBDeploymentOperationStore) ListDeploymentOperationRecords(ctx context.Context, applicationID string) ([]DeploymentRecord, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("mariadb deployment operation store is not configured")
	}
	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT `+deploymentOperationColumns+`
		FROM aods_deployment_operations
		WHERE application_id = ? AND status <> ?
		ORDER BY created_at DESC`,
		applicationID,
		string(DeploymentOperationCompleted),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []DeploymentRecord{}
	for rows.Next() {
		operation, err := scanDeploymentOperation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, operation.deploymentRecord())
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s MariaDBDeploymentOperationStore) GetDeploymentOperationRecord(ctx context.Context, applicationID string, deploymentID string) (DeploymentRecord, error) {
	if s.DB == nil {
		return DeploymentRecord{}, fmt.Errorf("mariadb deployment operation store is not configured")
	}
	row := s.DB.QueryRowContext(
		ctx,
		`SELECT `+deploymentOperationColumns+`
		FROM aods_deployment_operations
		WHERE id = ? AND application_id = ?`,
		deploymentID,
		applicationID,
	)
	operation, err := scanDeploymentOperation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return DeploymentRecord{}, ErrDeploymentNotFound
	}
	if err != nil {
		return DeploymentRecord{}, err
	}
	return operation.deploymentRecord(), nil
}

func (s MariaDBDeploymentOperationStore) ClaimNextDeploymentOperation(ctx context.Context, workerID string, leaseDuration time.Duration) (DeploymentOperation, bool, error) {
	if s.DB == nil {
		return DeploymentOperation{}, false, fmt.Errorf("mariadb deployment operation store is not configured")
	}
	if leaseDuration <= 0 {
		leaseDuration = 5 * time.Minute
	}
	now := timeNowUTC()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return DeploymentOperation{}, false, err
	}
	defer rollbackUnlessCommitted(tx)

	rows, err := tx.QueryContext(
		ctx,
		`SELECT `+deploymentOperationColumns+`
		FROM aods_deployment_operations
		WHERE status IN (?, ?) AND next_attempt_at <= ?
		ORDER BY created_at ASC
		LIMIT 10
		FOR UPDATE SKIP LOCKED`,
		string(DeploymentOperationQueued),
		string(DeploymentOperationRetrying),
		now,
	)
	if err != nil {
		return DeploymentOperation{}, false, err
	}

	candidates := []DeploymentOperation{}
	for rows.Next() {
		operation, err := scanDeploymentOperation(rows)
		if err != nil {
			_ = rows.Close()
			return DeploymentOperation{}, false, err
		}
		candidates = append(candidates, operation)
	}
	if err := rows.Close(); err != nil {
		return DeploymentOperation{}, false, err
	}
	if err := rows.Err(); err != nil {
		return DeploymentOperation{}, false, err
	}

	for _, operation := range candidates {
		acquired, err := acquireOperationLock(ctx, tx, operation.LockKey, workerID, now, now.Add(leaseDuration))
		if err != nil {
			return DeploymentOperation{}, false, err
		}
		if !acquired {
			continue
		}

		operation.Status = DeploymentOperationRunning
		operation.AttemptCount++
		operation.LeaseOwner = workerID
		operation.LeaseUntil = now.Add(leaseDuration)
		operation.Message = "배포 worker가 Git desired state 반영을 처리하고 있습니다."
		operation.UpdatedAt = now
		result, err := tx.ExecContext(
			ctx,
			`UPDATE aods_deployment_operations
			SET status = ?, message = ?, attempt_count = ?, lease_owner = ?, lease_until = ?, updated_at = ?, version = version + 1
			WHERE id = ? AND version = ?`,
			string(operation.Status),
			operation.Message,
			operation.AttemptCount,
			operation.LeaseOwner,
			operation.LeaseUntil,
			operation.UpdatedAt,
			operation.ID,
			operation.Version,
		)
		if err != nil {
			return DeploymentOperation{}, false, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return DeploymentOperation{}, false, err
		}
		if affected == 0 {
			continue
		}
		operation.Version++
		if err := tx.Commit(); err != nil {
			return DeploymentOperation{}, false, err
		}
		return operation, true, nil
	}

	if err := tx.Commit(); err != nil {
		return DeploymentOperation{}, false, err
	}
	return DeploymentOperation{}, false, nil
}

func (s MariaDBDeploymentOperationStore) MarkDeploymentOperationSucceeded(ctx context.Context, operation DeploymentOperation) error {
	now := timeNowUTC()
	return s.markOperationTerminal(ctx, operation, DeploymentOperationCompleted, "Git desired state 반영을 완료했습니다.", "", now)
}

func (s MariaDBDeploymentOperationStore) MarkDeploymentOperationRetry(ctx context.Context, operation DeploymentOperation, message string, nextAttemptAt time.Time) error {
	if s.DB == nil {
		return fmt.Errorf("mariadb deployment operation store is not configured")
	}
	now := timeNowUTC()
	status := DeploymentOperationRetrying
	if operation.AttemptCount >= maxDeploymentOperationAttempts(operation) {
		return s.MarkDeploymentOperationFailed(ctx, operation, message)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)

	if err := releaseOperationLock(ctx, tx, operation.LockKey, operation.LeaseOwner, now); err != nil {
		return err
	}
	result, err := tx.ExecContext(
		ctx,
		`UPDATE aods_deployment_operations
		SET status = ?, message = ?, last_error = ?, next_attempt_at = ?, lease_owner = NULL, lease_until = NULL,
			updated_at = ?, version = version + 1
		WHERE id = ? AND lease_owner = ?`,
		string(status),
		"외부 의존성 또는 Git write 실패 후 재시도를 기다리고 있습니다.",
		strings.TrimSpace(message),
		nextAttemptAt,
		now,
		operation.ID,
		operation.LeaseOwner,
	)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return fmt.Errorf("deployment operation %s lease was lost before retry state update", operation.ID)
	}
	return tx.Commit()
}

func (s MariaDBDeploymentOperationStore) MarkDeploymentOperationFailed(ctx context.Context, operation DeploymentOperation, message string) error {
	return s.markOperationTerminal(ctx, operation, DeploymentOperationFailed, "배포 operation이 재시도 한도를 초과했습니다.", strings.TrimSpace(message), timeNowUTC())
}

func (s MariaDBDeploymentOperationStore) markOperationTerminal(
	ctx context.Context,
	operation DeploymentOperation,
	status DeploymentOperationStatus,
	message string,
	lastError string,
	now time.Time,
) error {
	if s.DB == nil {
		return fmt.Errorf("mariadb deployment operation store is not configured")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)

	if err := releaseOperationLock(ctx, tx, operation.LockKey, operation.LeaseOwner, now); err != nil {
		return err
	}
	result, err := tx.ExecContext(
		ctx,
		`UPDATE aods_deployment_operations
		SET status = ?, message = ?, last_error = ?, lease_owner = NULL, lease_until = NULL,
			updated_at = ?, completed_at = ?, version = version + 1
		WHERE id = ? AND lease_owner = ?`,
		string(status),
		message,
		nullableString(lastError),
		now,
		now,
		operation.ID,
		operation.LeaseOwner,
	)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		return fmt.Errorf("deployment operation %s lease was lost before terminal state update", operation.ID)
	}
	return tx.Commit()
}

func acquireOperationLock(
	ctx context.Context,
	tx *sql.Tx,
	lockKey string,
	workerID string,
	now time.Time,
	leaseUntil time.Time,
) (bool, error) {
	if strings.TrimSpace(lockKey) == "" {
		return false, fmt.Errorf("operation lock key is required")
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT IGNORE INTO aods_operation_locks (lock_key, lease_owner, lease_until, version, updated_at)
		VALUES (?, '', ?, 1, ?)`,
		lockKey,
		time.Unix(0, 0).UTC(),
		now,
	); err != nil {
		return false, err
	}

	var currentOwner string
	var currentUntil time.Time
	var version int64
	err := tx.QueryRowContext(
		ctx,
		`SELECT lease_owner, lease_until, version FROM aods_operation_locks WHERE lock_key = ? FOR UPDATE`,
		lockKey,
	).Scan(&currentOwner, &currentUntil, &version)
	if err != nil {
		return false, err
	}
	if currentUntil.After(now) && currentOwner != workerID {
		return false, nil
	}
	_, err = tx.ExecContext(
		ctx,
		`UPDATE aods_operation_locks
		SET lease_owner = ?, lease_until = ?, updated_at = ?, version = version + 1
		WHERE lock_key = ? AND version = ?`,
		workerID,
		leaseUntil,
		now,
		lockKey,
		version,
	)
	return err == nil, err
}

func releaseOperationLock(ctx context.Context, tx *sql.Tx, lockKey string, workerID string, now time.Time) error {
	if strings.TrimSpace(lockKey) == "" || strings.TrimSpace(workerID) == "" {
		return nil
	}
	_, err := tx.ExecContext(
		ctx,
		`UPDATE aods_operation_locks
		SET lease_owner = '', lease_until = ?, updated_at = ?, version = version + 1
		WHERE lock_key = ? AND lease_owner = ?`,
		time.Unix(0, 0).UTC(),
		now,
		lockKey,
		workerID,
	)
	return err
}

func (s MariaDBDeploymentOperationStore) getOperation(ctx context.Context, id string) (DeploymentOperation, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+deploymentOperationColumns+` FROM aods_deployment_operations WHERE id = ?`, id)
	return scanDeploymentOperation(row)
}

const deploymentOperationColumns = `id, application_id, project_id, application_name, environment, image_tag,
	desired_image, deployment_strategy, requested_by, request_id, lock_key, status, message, attempt_count,
	max_attempts, last_error, next_attempt_at, lease_owner, lease_until, version, created_at, updated_at, completed_at`

type deploymentOperationScanner interface {
	Scan(dest ...any) error
}

func scanDeploymentOperation(scanner deploymentOperationScanner) (DeploymentOperation, error) {
	var operation DeploymentOperation
	var strategy string
	var status string
	var message sql.NullString
	var lastError sql.NullString
	var leaseOwner sql.NullString
	var leaseUntil sql.NullTime
	var completedAt sql.NullTime
	if err := scanner.Scan(
		&operation.ID,
		&operation.ApplicationID,
		&operation.ProjectID,
		&operation.ApplicationName,
		&operation.Environment,
		&operation.ImageTag,
		&operation.DesiredImage,
		&strategy,
		&operation.RequestedBy,
		&operation.RequestID,
		&operation.LockKey,
		&status,
		&message,
		&operation.AttemptCount,
		&operation.MaxAttempts,
		&lastError,
		&operation.NextAttemptAt,
		&leaseOwner,
		&leaseUntil,
		&operation.Version,
		&operation.CreatedAt,
		&operation.UpdatedAt,
		&completedAt,
	); err != nil {
		return DeploymentOperation{}, err
	}
	operation.DeploymentStrategy = NormalizeDeploymentStrategy(DeploymentStrategy(strategy))
	operation.Status = DeploymentOperationStatus(status)
	operation.Message = message.String
	operation.LastError = lastError.String
	operation.LeaseOwner = leaseOwner.String
	if leaseUntil.Valid {
		operation.LeaseUntil = leaseUntil.Time
	}
	if completedAt.Valid {
		operation.CompletedAt = &completedAt.Time
	}
	return operation, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func maxInt64(value int64, fallback int64) int64 {
	if value <= 0 {
		return fallback
	}
	return value
}

func rollbackUnlessCommitted(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
