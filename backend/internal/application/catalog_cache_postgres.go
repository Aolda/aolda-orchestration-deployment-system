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

type PostgresApplicationCatalogCache struct {
	DB *sql.DB
}

func (s PostgresApplicationCatalogCache) Ensure(ctx context.Context) error {
	if s.DB == nil {
		return fmt.Errorf("postgres application catalog cache is not configured")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS aods_application_catalog_snapshots (
			project_id TEXT NOT NULL PRIMARY KEY,
			record_count INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_aods_application_catalog_snapshots_updated
			ON aods_application_catalog_snapshots (updated_at)`,
		`CREATE TABLE IF NOT EXISTS aods_application_catalog_records (
			project_id TEXT NOT NULL,
			application_id TEXT NOT NULL,
			application_name TEXT NOT NULL,
			record_json JSONB NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (project_id, application_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_aods_application_catalog_project_name
			ON aods_application_catalog_records (project_id, application_name)`,
		`CREATE INDEX IF NOT EXISTS idx_aods_application_catalog_updated
			ON aods_application_catalog_records (updated_at)`,
	}
	for _, statement := range statements {
		if _, err := s.DB.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s PostgresApplicationCatalogCache) ListApplications(ctx context.Context, projectID string, maxAge time.Duration) ([]Record, bool, error) {
	if s.DB == nil {
		return nil, false, fmt.Errorf("postgres application catalog cache is not configured")
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
		WHERE project_id = $1`,
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
		WHERE project_id = $1
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

func (s PostgresApplicationCatalogCache) ReplaceProjectApplications(ctx context.Context, projectID string, records []Record) error {
	if s.DB == nil {
		return fmt.Errorf("postgres application catalog cache is not configured")
	}
	now := timeNowUTC()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackUnlessCommitted(tx)

	if _, err := tx.ExecContext(ctx, `DELETE FROM aods_application_catalog_records WHERE project_id = $1`, projectID); err != nil {
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
			) VALUES ($1, $2, $3, $4, $5)`,
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
		) VALUES ($1, $2, $3)
		ON CONFLICT (project_id) DO UPDATE SET
			record_count = EXCLUDED.record_count,
			updated_at = EXCLUDED.updated_at`,
		projectID,
		len(sorted),
		now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s PostgresApplicationCatalogCache) InvalidateProject(ctx context.Context, projectID string) error {
	if s.DB == nil {
		return fmt.Errorf("postgres application catalog cache is not configured")
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM aods_application_catalog_snapshots WHERE project_id = $1`, projectID)
	return err
}
