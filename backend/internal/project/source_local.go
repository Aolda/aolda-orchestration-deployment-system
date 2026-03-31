package project

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type LocalCatalogSource struct {
	Path string
}

type catalogFile struct {
	Projects []catalogProject `yaml:"projects"`
}

type catalogProject struct {
	ID          string        `yaml:"id"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description"`
	Namespace   string        `yaml:"namespace"`
	Access      catalogAccess `yaml:"access"`
}

type catalogAccess struct {
	ViewerGroups   []string `yaml:"viewerGroups"`
	DeployerGroups []string `yaml:"deployerGroups"`
	AdminGroups    []string `yaml:"adminGroups"`
}

func (s LocalCatalogSource) ListProjects(ctx context.Context) ([]CatalogProject, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, fmt.Errorf("read project catalog: %w", err)
	}

	var file catalogFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode project catalog: %w", err)
	}

	projects := make([]CatalogProject, 0, len(file.Projects))
	for _, item := range file.Projects {
		projects = append(projects, CatalogProject{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			Namespace:   item.Namespace,
			Access: Access{
				ViewerGroups:   item.Access.ViewerGroups,
				DeployerGroups: item.Access.DeployerGroups,
				AdminGroups:    item.Access.AdminGroups,
			},
		})
	}

	return projects, nil
}
