package cluster

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/aolda/aods-backend/internal/gitops"
)

type GitSource struct {
	Repository                 *gitops.Repository
	FluxKustomizationNamespace string
	FluxSourceName             string
}

func (s GitSource) ListClusters(ctx context.Context) ([]Summary, error) {
	if s.Repository == nil {
		return nil, fmt.Errorf("git cluster repository is not configured")
	}

	var items []Summary
	err := s.Repository.WithRead(ctx, func(repoDir string) error {
		clusters, err := s.localSource(repoDir).ListClusters(ctx)
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

func (s GitSource) CreateCluster(ctx context.Context, input CreateRequest) (Summary, error) {
	if s.Repository == nil {
		return Summary{}, fmt.Errorf("git cluster repository is not configured")
	}

	var created Summary
	err := s.Repository.WithWrite(ctx, fmt.Sprintf("feat: bootstrap cluster %s", input.ID), func(repoDir string) error {
		cluster, err := s.localSource(repoDir).CreateCluster(ctx, input)
		if err != nil {
			return err
		}
		created = cluster
		return nil
	})
	if err != nil {
		return Summary{}, err
	}
	return created, nil
}

func (s GitSource) localSource(repoDir string) LocalSource {
	return LocalSource{
		Path:                       filepath.Join(repoDir, "platform", "clusters.yaml"),
		RepoRoot:                   repoDir,
		FluxKustomizationNamespace: s.FluxKustomizationNamespace,
		FluxSourceName:             s.FluxSourceName,
	}
}
