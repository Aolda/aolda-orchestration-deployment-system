package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type MariaDBApplicationCatalogCache struct {
	DB *sql.DB
}

func (s MariaDBApplicationCatalogCache) Ensure(ctx context.Context) error {
	if s.DB == nil {
		return fmt.Errorf("mariadb application catalog cache is not configured")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS aods_application_catalog_snapshots (
			project_id VARCHAR(128) NOT NULL PRIMARY KEY,
			record_count INT NOT NULL DEFAULT 0,
			updated_at DATETIME(6) NOT NULL,
			KEY idx_aods_application_catalog_snapshots_updated (updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS aods_application_catalog_records (
			project_id VARCHAR(128) NOT NULL,
			application_id VARCHAR(255) NOT NULL,
			application_name VARCHAR(128) NOT NULL,
			record_json LONGTEXT NOT NULL,
			updated_at DATETIME(6) NOT NULL,
			PRIMARY KEY (project_id, application_id),
			KEY idx_aods_application_catalog_project_name (project_id, application_name),
			KEY idx_aods_application_catalog_updated (updated_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, statement := range statements {
		if _, err := s.DB.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s MariaDBApplicationCatalogCache) ListApplications(ctx context.Context, projectID string, maxAge time.Duration) ([]Record, bool, error) {
	if s.DB == nil {
		return nil, false, fmt.Errorf("mariadb application catalog cache is not configured")
	}
	if maxAge <= 0 {
		return nil, false, nil
	}

	var snapshotUpdatedAt time.Time
	var recordCount int
	err := s.DB.QueryRowContext(
		ctx,
		`SELECT updated_at, record_count
		FROM aods_application_catalog_snapshots
		WHERE project_id = ?`,
		projectID,
	).Scan(&snapshotUpdatedAt, &recordCount)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if timeNowUTC().Sub(snapshotUpdatedAt) > maxAge {
		return nil, false, nil
	}
	if recordCount == 0 {
		return []Record{}, true, nil
	}

	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT record_json
		FROM aods_application_catalog_records
		WHERE project_id = ?
		ORDER BY application_name ASC, application_id ASC`,
		projectID,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	records := []Record{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, false, err
		}
		var record Record
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, false, fmt.Errorf("decode cached application record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(records) != recordCount {
		return nil, false, nil
	}
	return records, true, nil
}

func (s MariaDBApplicationCatalogCache) ReplaceProjectApplications(ctx context.Context, projectID string, records []Record) error {
	if s.DB == nil {
		return fmt.Errorf("mariadb application catalog cache is not configured")
	}
	now := timeNowUTC()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)

	if _, err := tx.ExecContext(ctx, `DELETE FROM aods_application_catalog_records WHERE project_id = ?`, projectID); err != nil {
		return err
	}
	sorted := append([]Record(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Name == sorted[j].Name {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Name < sorted[j].Name
	})
	for _, record := range sorted {
		raw, err := json.Marshal(record)
		if err != nil {
			return fmt.Errorf("encode cached application record: %w", err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO aods_application_catalog_records (
				project_id, application_id, application_name, record_json, updated_at
			) VALUES (?, ?, ?, ?, ?)`,
			projectID,
			record.ID,
			record.Name,
			string(raw),
			now,
		); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO aods_application_catalog_snapshots (
			project_id, record_count, updated_at
		) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE
			record_count = VALUES(record_count),
			updated_at = VALUES(updated_at)`,
		projectID,
		len(sorted),
		now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s MariaDBApplicationCatalogCache) InvalidateProject(ctx context.Context, projectID string) error {
	if s.DB == nil {
		return fmt.Errorf("mariadb application catalog cache is not configured")
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM aods_application_catalog_snapshots WHERE project_id = ?`, projectID)
	return err
}
