package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aolda/aods-backend/internal/fluxscaffold"
	"gopkg.in/yaml.v3"
)

type LocalSource struct {
	Path                       string
	RepoRoot                   string
	FluxKustomizationNamespace string
	FluxSourceName             string
}

type catalogFile struct {
	Clusters []catalogCluster `yaml:"clusters"`
}

type catalogCluster struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Default     bool   `yaml:"default"`
}

func (s LocalSource) ListClusters(ctx context.Context) ([]Summary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	file, _, _, err := s.readCatalogFile()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Summary{defaultCluster()}, nil
		}
		return nil, err
	}

	items := clusterSummaries(normalizeCatalogClusters(file.Clusters))
	if len(items) == 0 {
		return []Summary{defaultCluster()}, nil
	}
	return items, nil
}

func (s LocalSource) CreateCluster(ctx context.Context, input CreateRequest) (Summary, error) {
	if err := ctx.Err(); err != nil {
		return Summary{}, err
	}

	file, originalData, existed, err := s.readCatalogFile()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Summary{}, err
	}

	clusterID := strings.TrimSpace(input.ID)
	for _, item := range file.Clusters {
		if strings.TrimSpace(item.ID) == clusterID {
			return Summary{}, ErrConflict
		}
	}
	if input.Default {
		for index := range file.Clusters {
			file.Clusters[index].Default = false
		}
	}

	file.Clusters = append(file.Clusters, catalogCluster{
		ID:          clusterID,
		Name:        strings.TrimSpace(input.Name),
		Description: strings.TrimSpace(input.Description),
		Default:     input.Default,
	})
	file.Clusters = normalizeCatalogClusters(file.Clusters)

	rendered, err := yaml.Marshal(&file)
	if err != nil {
		return Summary{}, fmt.Errorf("encode cluster catalog: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return Summary{}, fmt.Errorf("create cluster catalog directory: %w", err)
	}
	if err := os.WriteFile(s.Path, rendered, 0o644); err != nil {
		return Summary{}, fmt.Errorf("write cluster catalog: %w", err)
	}

	if err := fluxscaffold.EnsureCluster(fluxscaffold.Config{
		RepoRoot:               s.repoRoot(),
		ClusterID:              clusterID,
		KustomizationNamespace: s.fluxKustomizationNamespace(),
		SourceName:             s.fluxSourceName(),
	}); err != nil {
		_ = s.rollbackCatalogWrite(originalData, existed)
		_ = s.removeClusterArtifacts(clusterID)
		return Summary{}, err
	}

	for _, item := range clusterSummaries(file.Clusters) {
		if item.ID == clusterID {
			return item, nil
		}
	}

	return Summary{}, fmt.Errorf("created cluster %s could not be loaded", clusterID)
}

func (s LocalSource) readCatalogFile() (catalogFile, []byte, bool, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return catalogFile{}, nil, false, err
		}
		return catalogFile{}, nil, false, fmt.Errorf("read cluster catalog: %w", err)
	}

	var file catalogFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return catalogFile{}, nil, false, fmt.Errorf("decode cluster catalog: %w", err)
	}

	return file, data, true, nil
}

func (s LocalSource) rollbackCatalogWrite(originalData []byte, existed bool) error {
	if !existed {
		if err := os.Remove(s.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return os.WriteFile(s.Path, originalData, 0o644)
}

func (s LocalSource) removeClusterArtifacts(clusterID string) error {
	for _, path := range []string{
		fluxscaffold.BootstrapDir(s.repoRoot(), clusterID),
		fluxscaffold.ClusterDir(s.repoRoot(), clusterID),
	} {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func (s LocalSource) repoRoot() string {
	if root := strings.TrimSpace(s.RepoRoot); root != "" {
		return root
	}
	return filepath.Dir(filepath.Dir(s.Path))
}

func (s LocalSource) fluxKustomizationNamespace() string {
	if namespace := strings.TrimSpace(s.FluxKustomizationNamespace); namespace != "" {
		return namespace
	}
	return fluxscaffold.DefaultKustomizationNamespace
}

func (s LocalSource) fluxSourceName() string {
	if sourceName := strings.TrimSpace(s.FluxSourceName); sourceName != "" {
		return sourceName
	}
	return fluxscaffold.DefaultSourceName
}

func clusterSummaries(clusters []catalogCluster) []Summary {
	items := make([]Summary, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.ID == "" {
			continue
		}
		item := Summary{
			ID:          cluster.ID,
			Name:        cluster.Name,
			Description: cluster.Description,
			Default:     cluster.Default,
		}
		if item.Name == "" {
			item.Name = item.ID
		}
		items = append(items, item)
	}
	return items
}

func normalizeCatalogClusters(clusters []catalogCluster) []catalogCluster {
	items := make([]catalogCluster, 0, len(clusters))
	defaultIndex := -1
	for _, cluster := range clusters {
		cluster.ID = strings.TrimSpace(cluster.ID)
		cluster.Name = strings.TrimSpace(cluster.Name)
		cluster.Description = strings.TrimSpace(cluster.Description)
		if cluster.ID == "" {
			continue
		}
		if cluster.Name == "" {
			cluster.Name = cluster.ID
		}
		if cluster.Default && defaultIndex == -1 {
			defaultIndex = len(items)
		} else {
			cluster.Default = false
		}
		items = append(items, cluster)
	}
	if len(items) == 0 {
		return items
	}
	if defaultIndex == -1 {
		items[0].Default = true
		return items
	}
	for index := range items {
		items[index].Default = index == defaultIndex
	}
	return items
}
