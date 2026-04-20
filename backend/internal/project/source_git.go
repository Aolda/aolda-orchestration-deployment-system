package project

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aolda/aods-backend/internal/gitops"
)

type GitCatalogSource struct {
	Repository                 *gitops.Repository
	FluxKustomizationNamespace string
	FluxSourceName             string
}

func (s GitCatalogSource) ListProjects(ctx context.Context) ([]CatalogProject, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git catalog repository is not configured")
	}

	var projects []CatalogProject
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		items, err := s.localSource(repoDir).ListProjects(ctx)
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

func (s GitCatalogSource) CreateProject(ctx context.Context, input CreateRequest) (CatalogProject, error) {
	if s.Repository == nil {
		return CatalogProject{}, fmt.Errorf("git catalog repository is not configured")
	}

	var created CatalogProject
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: bootstrap project %s", input.ID), func(repoDir string) error {
		item, err := s.localSource(repoDir).CreateProject(ctx, input)
		if err != nil {
			return err
		}
		created = item
		return nil
	})
	if err != nil {
		return CatalogProject{}, err
	}
	return created, nil
}

func (s GitCatalogSource) DeleteProject(ctx context.Context, projectID string) (LifecycleResponse, error) {
	if s.Repository == nil {
		return LifecycleResponse{}, fmt.Errorf("git catalog repository is not configured")
	}

	var deleted LifecycleResponse
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: delete project %s", projectID), func(repoDir string) error {
		item, err := s.localSource(repoDir).DeleteProject(ctx, projectID)
		if err != nil {
			var missing CatalogNotFoundError
			if errors.As(err, &missing) {
				return CatalogNotFoundError{Path: filepath.Join("platform", "projects.yaml")}
			}
			return err
		}
		deleted = item
		return nil
	})
	if err != nil {
		return LifecycleResponse{}, err
	}

	return deleted, nil
}

func (s GitCatalogSource) UpdatePolicies(ctx context.Context, projectID string, policies PolicySet) (CatalogProject, error) {
	if s.Repository == nil {
		return CatalogProject{}, fmt.Errorf("git catalog repository is not configured")
	}

	var updated CatalogProject
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: update policies for %s", projectID), func(repoDir string) error {
		project, err := s.localSource(repoDir).UpdatePolicies(ctx, projectID, policies)
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

func (s GitCatalogSource) localSource(repoDir string) LocalCatalogSource {
	return LocalCatalogSource{
		Path:                       filepath.Join(repoDir, "platform", "projects.yaml"),
		RepoRoot:                   repoDir,
		FluxKustomizationNamespace: s.FluxKustomizationNamespace,
		FluxSourceName:             s.FluxSourceName,
	}
}
