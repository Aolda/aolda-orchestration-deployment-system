package application

import (
	"context"
	"fmt"

	"github.com/aolda/aods-backend/internal/gitops"
)

type GitManifestStore struct {
	Repository *gitops.Repository
}

func (s GitManifestStore) ListApplications(ctx context.Context, projectID string) ([]Record, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git manifest repository is not configured")
	}

	var records []Record
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		items, err := LocalManifestStore{RepoRoot: repoDir}.ListApplications(ctx, projectID)
		if err != nil {
			return err
		}
		records = items
		return nil
	})
	if err != nil {
		return nil, err
	}

	return records, nil
}

func (s GitManifestStore) GetApplication(ctx context.Context, applicationID string) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git manifest repository is not configured")
	}

	var record Record
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		item, err := LocalManifestStore{RepoRoot: repoDir}.GetApplication(ctx, applicationID)
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

func (s GitManifestStore) CreateApplication(
	ctx context.Context,
	project ProjectContext,
	input CreateRequest,
	secretPath string,
) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git manifest repository is not configured")
	}

	var record Record
	err := s.Repository.WithWrite(
		ctx,
		fmt.Sprintf("feat: create application %s in %s", input.Name, project.ID),
		func(repoDir string) error {
			item, err := LocalManifestStore{RepoRoot: repoDir}.CreateApplication(ctx, project, input, secretPath)
			if err != nil {
				return err
			}
			record = item
			return nil
		},
	)
	if err != nil {
		return Record{}, err
	}

	return record, nil
}

func (s GitManifestStore) UpdateApplicationImage(
	ctx context.Context,
	applicationID string,
	imageTag string,
) (Record, error) {
	if s.Repository == nil {
		return Record{}, fmt.Errorf("git manifest repository is not configured")
	}

	var record Record
	err := s.Repository.WithWrite(
		ctx,
		fmt.Sprintf("feat: redeploy %s with image tag %s", applicationID, imageTag),
		func(repoDir string) error {
			item, err := LocalManifestStore{RepoRoot: repoDir}.UpdateApplicationImage(ctx, applicationID, imageTag)
			if err != nil {
				return err
			}
			record = item
			return nil
		},
	)
	if err != nil {
		return Record{}, err
	}

	return record, nil
}
