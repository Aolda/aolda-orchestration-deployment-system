package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type LocalSource struct {
	Path string
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

	data, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []Summary{defaultCluster()}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cluster catalog: %w", err)
	}

	var file catalogFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode cluster catalog: %w", err)
	}

	items := make([]Summary, 0, len(file.Clusters))
	hasDefault := false
	for _, cluster := range file.Clusters {
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
		if item.Default {
			hasDefault = true
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return []Summary{defaultCluster()}, nil
	}
	if !hasDefault {
		items[0].Default = true
	}
	return items, nil
}
