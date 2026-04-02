package change

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type LocalStore struct {
	RepoRoot string
}

func (s LocalStore) Create(ctx context.Context, record Record) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	return s.Update(ctx, record)
}

func (s LocalStore) Get(ctx context.Context, changeID string) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(s.path(changeID))
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, fmt.Errorf("read change: %w", err)
	}
	var record Record
	if err := yaml.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("decode change: %w", err)
	}
	return record, nil
}

func (s LocalStore) Update(ctx context.Context, record Record) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	if err := os.MkdirAll(filepath.Dir(s.path(record.ID)), 0o755); err != nil {
		return Record{}, fmt.Errorf("create change directory: %w", err)
	}
	data, err := yaml.Marshal(record)
	if err != nil {
		return Record{}, fmt.Errorf("encode change: %w", err)
	}
	if err := os.WriteFile(s.path(record.ID), data, 0o644); err != nil {
		return Record{}, fmt.Errorf("write change: %w", err)
	}
	return record, nil
}

func (s LocalStore) path(changeID string) string {
	return filepath.Join(s.RepoRoot, "platform", "changes", changeID+".yaml")
}
