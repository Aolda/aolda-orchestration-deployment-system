package cluster

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aolda/aods-backend/internal/gitops"
)

type GitSource struct {
	Repository *gitops.Repository
}

func (s GitSource) ListClusters(ctx context.Context) ([]Summary, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git cluster repository is not configured")
	}

	var items []Summary
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		clusters, err := LocalSource{Path: filepath.Join(repoDir, "platform", "clusters.yaml")}.ListClusters(ctx)
		if err != nil {
			return err
		}
		items = clusters
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}
