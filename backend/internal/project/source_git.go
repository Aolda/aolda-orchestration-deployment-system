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
