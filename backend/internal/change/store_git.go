package change

import (
	"context"
	"fmt"

	"github.com/aolda/aods-backend/internal/gitops"
)

type GitStore struct {
	Repository *gitops.Repository
}

func (s GitStore) Create(ctx context.Context, record Record) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git change repository is not configured")
	}
	var created Record
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: create change %s", record.ID), func(repoDir string) error {
		item, err := LocalStore{RepoRoot: repoDir}.Create(ctx, record)
		if err != nil {
			return err
		}
		created = item
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return created, nil
}

func (s GitStore) Get(ctx context.Context, changeID string) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git change repository is not configured")
	}
	var record Record
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		item, err := LocalStore{RepoRoot: repoDir}.Get(ctx, changeID)
		if err != nil {
			return err
		}
		record = item
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s GitStore) Update(ctx context.Context, record Record) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git change repository is not configured")
	}
	var updated Record
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: update change %s", record.ID), func(repoDir string) error {
		item, err := LocalStore{RepoRoot: repoDir}.Update(ctx, record)
		if err != nil {
			return err
		}
		updated = item
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return updated, nil
}
