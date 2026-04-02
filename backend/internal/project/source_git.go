package project

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aolda/aods-backend/internal/gitops"
)

type GitCatalogSource struct {
	Repository *gitops.Repository
}

func (s GitCatalogSource) ListProjects(ctx context.Context) ([]CatalogProject, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git catalog repository is not configured")
	}

	var projects []CatalogProject
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		items, err := LocalCatalogSource{
			Path: filepath.Join(repoDir, "platform", "projects.yaml"),
		}.ListProjects(ctx)
		if err != nil {
			var missing CatalogNotFoundError
			if errors.As(err, &missing) {
				return CatalogNotFoundError{Path: filepath.Join("platform", "projects.yaml")}
			}
			return err
		}
		projects = items
		return nil
	})
	if err != nil {
		return nil, err
	}

	return projects, nil
}

func (s GitCatalogSource) UpdatePolicies(ctx context.Context, projectID string, policies PolicySet) (CatalogProject, error) {
	if s.Repository == nil {
		return CatalogProject{}, fmt.Errorf("git catalog repository is not configured")
	}

	var updated CatalogProject
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: update policies for %s", projectID), func(repoDir string) error {
		project, err := LocalCatalogSource{
			Path: filepath.Join(repoDir, "platform", "projects.yaml"),
		}.UpdatePolicies(ctx, projectID, policies)
		if err != nil {
			var missing CatalogNotFoundError
			if errors.As(err, &missing) {
				return CatalogNotFoundError{Path: filepath.Join("platform", "projects.yaml")}
			}
			return err
		}
		updated = project
		return nil
	})
	if err != nil {
		return CatalogProject{}, err
	}

	return updated, nil
}
